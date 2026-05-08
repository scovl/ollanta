package e2e_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	scannerToken = "ollanta-dev-scanner-token"
	adminLogin   = "admin"
	adminPass    = "admin"
)

var (
	scannerBinOnce sync.Once
	scannerBinPath string
	scannerBinErr  error
)

// TestE2E_ScanPipeline runs the complete scanner → server → ingestion pipeline
// inside a disposable Docker Compose stack. It is skippable when Docker is unavailable.
func TestE2E_ScanPipeline(t *testing.T) {
	requireDocker(t)

	repoRoot := findRepoRoot(t)
	scannerBin := buildScannerOnce(t, repoRoot)

	composeProject := fmt.Sprintf("ollantae2e%d", rand.Int())
	baseURL, cleanup := startStack(t, repoRoot, composeProject)
	defer cleanup()

	waitForReady(t, baseURL, 60*time.Second)

	adminJWT := loginAdmin(t, baseURL)

	// ── Happy path ──────────────────────────────────────────────────────
	t.Run("HappyPath", func(t *testing.T) {
		projectKey := "e2e-happy"
		createProject(t, baseURL, adminJWT, projectKey, "E2E Happy")

		fixtureDir := createFixture(t)
		reportJSON := runScanner(t, scannerBin, fixtureDir, projectKey)

		// Push report (gzip)
		body := pushReport(t, baseURL, scannerToken, reportJSON, "")
		jobID := int64(body["id"].(float64))

		job := pollScanJob(t, baseURL, scannerToken, jobID, 60*time.Second)
		if job["status"] != "completed" {
			t.Fatalf("scan job did not complete: %+v", job)
		}

		// Issues
		issues := listIssues(t, baseURL, adminJWT, projectKey)
		if len(issues) < 1 {
			t.Fatalf("expected at least 1 issue, got %d", len(issues))
		}

		// Quality gate evaluated
		overview := getOverview(t, baseURL, adminJWT, projectKey)
		qg, ok := overview["quality_gate"].(map[string]interface{})
		if !ok || qg["status"] == "" {
			t.Fatal("quality gate not evaluated")
		}

		// Measures present
		points := getMeasuresTrend(t, baseURL, adminJWT, projectKey, "bugs")
		if len(points) < 1 {
			t.Fatalf("expected measures trend points, got %d", len(points))
		}
	})

	// ── Duplicate push idempotency ──────────────────────────────────────
	t.Run("DuplicatePush", func(t *testing.T) {
		projectKey := "e2e-dup"
		createProject(t, baseURL, adminJWT, projectKey, "E2E Dup")

		fixtureDir := createFixture(t)
		reportJSON := runScanner(t, scannerBin, fixtureDir, projectKey)

		idempotencyKey := "e2e-dup-key-001"

		// First push → accepted (202)
		body1 := pushReport(t, baseURL, scannerToken, reportJSON, idempotencyKey)
		if s := int(body1["status_code"].(float64)); s != http.StatusAccepted {
			t.Fatalf("first push expected 202, got %d", s)
		}
		jobID1 := int64(body1["id"].(float64))
		pollScanJob(t, baseURL, scannerToken, jobID1, 60*time.Second)

		// Second push with same key → 200 duplicate
		body2 := pushReport(t, baseURL, scannerToken, reportJSON, idempotencyKey)
		if s := int(body2["status_code"].(float64)); s != http.StatusOK {
			t.Fatalf("duplicate push expected 200, got %d", s)
		}

		scans := listScans(t, baseURL, adminJWT, projectKey)
		if len(scans) != 1 {
			t.Fatalf("expected 1 scan after duplicate push, got %d", len(scans))
		}
	})

	// ── Push without auth ──────────────────────────────────────────────
	t.Run("NoAuth", func(t *testing.T) {
		fixtureDir := createFixture(t)
		reportJSON := runScanner(t, scannerBin, fixtureDir, "e2e-noauth")

		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/scans", bytes.NewReader(reportJSON))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("push request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	// ── Failed gate + webhook delivery ──────────────────────────────────
	t.Run("FailedGate", func(t *testing.T) {
		projectKey := "e2e-gate"
		project := createProject(t, baseURL, adminJWT, projectKey, "E2E Gate")
		projectID := int64(project["id"].(float64))

		// Create a strict gate (default conditions already include bugs > 0)
		gate := createQualityGate(t, baseURL, adminJWT, "E2E Strict Gate")
		gateID := int64(gate["id"].(float64))
		assignQualityGate(t, baseURL, adminJWT, projectKey, gateID)

		// Register a webhook so delivery is attempted
		wh := createWebhook(t, baseURL, adminJWT, projectID, "http://localhost:19999/noop")
		whID := int64(wh["id"].(float64))

		fixtureDir := createFixture(t)
		reportJSON := runScanner(t, scannerBin, fixtureDir, projectKey)

		body := pushReport(t, baseURL, scannerToken, reportJSON, "")
		jobID := int64(body["id"].(float64))
		job := pollScanJob(t, baseURL, scannerToken, jobID, 60*time.Second)

		if job["status"] != "completed" {
			t.Fatalf("scan job did not complete: %+v", job)
		}

		// Assert gate failed
		overview := getOverview(t, baseURL, adminJWT, projectKey)
		qg, ok := overview["quality_gate"].(map[string]interface{})
		if !ok || qg["status"] != "ERROR" {
			t.Fatalf("expected gate status ERROR, got %+v", qg)
		}

		// Wait for webhook delivery to be recorded
		var deliveries []map[string]interface{}
		for start := time.Now(); time.Since(start) < 30*time.Second; time.Sleep(1 * time.Second) {
			deliveries = listDeliveries(t, baseURL, adminJWT, whID)
			if len(deliveries) > 0 {
				break
			}
		}
		if len(deliveries) == 0 {
			t.Fatal("expected at least 1 webhook delivery")
		}
	})
}

// ── Docker / stack helpers ──────────────────────────────────────────────

func requireDocker(t *testing.T) {
	t.Helper()
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		t.Skip("docker not available:", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	// tests/e2e/scan_pipeline_test.go → go up three levels
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	return root
}

func buildScannerOnce(t *testing.T, repoRoot string) string {
	t.Helper()
	scannerBinOnce.Do(func() {
		tmpDir := t.TempDir()
		binName := "ollanta-scanner"
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		scannerBinPath = filepath.Join(tmpDir, binName)

		cmd := exec.Command("go", "build", "-o", scannerBinPath, "./ollantascanner/cmd/ollanta")
		cmd.Dir = repoRoot
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			scannerBinErr = fmt.Errorf("build scanner: %w\n%s", err, out)
		}
	})
	if scannerBinErr != nil {
		t.Fatal(scannerBinErr)
	}
	return scannerBinPath
}

func startStack(t *testing.T, repoRoot, projectName string) (baseURL string, cleanup func()) {
	t.Helper()

	env := append(os.Environ(), "COMPOSE_PROJECT_NAME="+projectName)

	t.Logf("starting docker compose stack (project=%s)", projectName)
	up := exec.Command("docker", "compose", "--profile", "server", "up", "-d", "--build")
	up.Dir = repoRoot
	up.Env = env
	if out, err := up.CombinedOutput(); err != nil {
		t.Fatalf("docker compose up failed: %v\n%s", err, out)
	}

	cleanup = func() {
		t.Logf("tearing down docker compose stack (project=%s)", projectName)
		down := exec.Command("docker", "compose", "--profile", "server", "down", "-v", "--remove-orphans")
		down.Dir = repoRoot
		down.Env = env
		_ = down.Run()
	}

	// Discover mapped port for ollantaweb:8080
	hostPort := getMappedPort(t, repoRoot, projectName, "ollantaweb", "8080")
	baseURL = "http://localhost:" + hostPort
	t.Logf("server baseURL: %s", baseURL)
	return baseURL, cleanup
}

func getMappedPort(t *testing.T, repoRoot, projectName, service, containerPort string) string {
	t.Helper()
	env := append(os.Environ(), "COMPOSE_PROJECT_NAME="+projectName)

	var out []byte
	for i := 0; i < 30; i++ {
		cmd := exec.Command("docker", "compose", "port", service, containerPort)
		cmd.Dir = repoRoot
		cmd.Env = env
		out, _ = cmd.CombinedOutput()
		if len(out) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	s := strings.TrimSpace(string(out))
	idx := strings.LastIndex(s, ":")
	if idx < 0 || idx+1 >= len(s) {
		t.Fatalf("could not determine mapped port for %s:%s: %q", service, containerPort, s)
	}
	port := s[idx+1:]
	port = strings.TrimSuffix(port, "/tcp")
	return port
}

func waitForReady(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			t.Logf("server ready")
			// Give workers a moment to start polling
			time.Sleep(3 * time.Second)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("server did not become ready within %v", timeout)
}

// ── API helpers ─────────────────────────────────────────────────────────

func loginAdmin(t *testing.T, baseURL string) string {
	t.Helper()
	payload := map[string]string{"login": adminLogin, "password": adminPass}
	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/auth/login", "", payload)
	return resp["access_token"].(string)
}

func createProject(t *testing.T, baseURL, token, key, name string) map[string]interface{} {
	t.Helper()
	return doRequest(t, http.MethodPost, baseURL+"/api/v1/projects", token, map[string]string{
		"key":  key,
		"name": name,
	})
}

func createQualityGate(t *testing.T, baseURL, token, name string) map[string]interface{} {
	t.Helper()
	return doRequest(t, http.MethodPost, baseURL+"/api/v1/quality-gates", token, map[string]string{
		"name": name,
	})
}

func assignQualityGate(t *testing.T, baseURL, token, projectKey string, gateID int64) {
	t.Helper()
	doRequest(t, http.MethodPost, baseURL+"/api/v1/projects/"+projectKey+"/quality-gate", token, map[string]interface{}{
		"gate_id": gateID,
	})
}

func createWebhook(t *testing.T, baseURL, token string, projectID int64, url string) map[string]interface{} {
	t.Helper()
	return doRequest(t, http.MethodPost, baseURL+"/api/v1/webhooks", token, map[string]interface{}{
		"project_id": projectID,
		"name":       "e2e-webhook",
		"url":        url,
		"events":     []string{"scan.completed"},
	})
}

func listIssues(t *testing.T, baseURL, token, projectKey string) []map[string]interface{} {
	t.Helper()
	resp := doRequest(t, http.MethodGet, baseURL+"/api/v1/issues?project_key="+projectKey, token, nil)
	items, _ := resp["items"].([]interface{})
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		out = append(out, it.(map[string]interface{}))
	}
	return out
}

func listScans(t *testing.T, baseURL, token, projectKey string) []map[string]interface{} {
	t.Helper()
	resp := doRequest(t, http.MethodGet, baseURL+"/api/v1/projects/"+projectKey+"/scans", token, nil)
	items, _ := resp["items"].([]interface{})
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		out = append(out, it.(map[string]interface{}))
	}
	return out
}

func getOverview(t *testing.T, baseURL, token, projectKey string) map[string]interface{} {
	t.Helper()
	return doRequest(t, http.MethodGet, baseURL+"/api/v1/projects/"+projectKey+"/overview", token, nil)
}

func getMeasuresTrend(t *testing.T, baseURL, token, projectKey, metric string) []map[string]interface{} {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/projects/%s/measures/trend?metric=%s", baseURL, projectKey, metric)
	resp := doRequest(t, http.MethodGet, url, token, nil)
	pts, _ := resp["points"].([]interface{})
	out := make([]map[string]interface{}, 0, len(pts))
	for _, p := range pts {
		out = append(out, p.(map[string]interface{}))
	}
	return out
}

func listDeliveries(t *testing.T, baseURL, token string, webhookID int64) []map[string]interface{} {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/webhooks/%d/deliveries", baseURL, webhookID)
	resp := doRequest(t, http.MethodGet, url, token, nil)
	items, _ := resp["items"].([]interface{})
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		out = append(out, it.(map[string]interface{}))
	}
	return out
}

// doRequest performs an HTTP request, decodes JSON, and returns the body map.
// For requests that return 204 No Content, an empty map is returned.
func doRequest(t *testing.T, method, url, token string, body any) map[string]interface{} {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return map[string]interface{}{}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(b))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result["status_code"] = float64(resp.StatusCode)
	return result
}

// ── Scanner helpers ─────────────────────────────────────────────────────

func createFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Go file: triggers go:useless-eqeq (bug) and go:no-large-functions (code smell)
	goSrc := `package main

func main() {
	a := 1
	_ = a == a
}

func LargeFunctionGo() {
	_ = "l01"; _ = "l02"; _ = "l03"; _ = "l04"; _ = "l05"
	_ = "l06"; _ = "l07"; _ = "l08"; _ = "l09"; _ = "l10"
	_ = "l11"; _ = "l12"; _ = "l13"; _ = "l14"; _ = "l15"
	_ = "l16"; _ = "l17"; _ = "l18"; _ = "l19"; _ = "l20"
	_ = "l21"; _ = "l22"; _ = "l23"; _ = "l24"; _ = "l25"
	_ = "l26"; _ = "l27"; _ = "l28"; _ = "l29"; _ = "l30"
	_ = "l31"; _ = "l32"; _ = "l33"; _ = "l34"; _ = "l35"
	_ = "l36"; _ = "l37"; _ = "l38"; _ = "l39"; _ = "l40"
	_ = "l41"; _ = "l42"; _ = "l43"; _ = "l44"; _ = "l45"
}
`
	writeFile(t, filepath.Join(dir, "main.go"), goSrc)

	// JavaScript file: triggers js:useless-eqeq (bug) and js:no-large-functions (code smell)
	jsSrc := `function test() {
    var x = 1;
    return x == x;
}

function largeFunctionJS() {
    var a = 1; var b = 2; var c = 3; var d = 4; var e = 5;
    var f = 6; var g = 7; var h = 8; var i = 9; var j = 10;
    var k = 11; var l = 12; var m = 13; var n = 14; var o = 15;
    var p = 16; var q = 17; var r = 18; var s = 19; var t = 20;
    var u = 21; var v = 22; var w = 23; var x = 24; var y = 25;
    var z = 26; var aa = 27; var bb = 28; var cc = 29; var dd = 30;
    var ee = 31; var ff = 32; var gg = 33; var hh = 34; var ii = 35;
    var jj = 36; var kk = 37; var ll = 38; var mm = 39; var nn = 40;
    var oo = 41; var pp = 42; var qq = 43; var rr = 44; var ss = 45;
    return a + b + c + d + e + f + g + h + i + j + k + l + m + n + o + p + q + r + s + t + u + v + w + x + y + z;
}
`
	writeFile(t, filepath.Join(dir, "app.js"), jsSrc)

	// Python file: triggers py:broad-except (bug), py:useless-eqeq (bug) and py:no-large-functions (code smell)
	pySrc := `try:
    pass
except Exception:
    pass

def useless():
    a = 1
    return a == a

def large_function_py():
    a = 1; b = 2; c = 3; d = 4; e = 5
    f = 6; g = 7; h = 8; i = 9; j = 10
    k = 11; l = 12; m = 13; n = 14; o = 15
    p = 16; q = 17; r = 18; s = 19; t = 20
    u = 21; v = 22; w = 23; x = 24; y = 25
    z = 26; aa = 27; bb = 28; cc = 29; dd = 30
    ee = 31; ff = 32; gg = 33; hh = 34; ii = 35
    jj = 36; kk = 37; ll = 38; mm = 39; nn = 40
    oo = 41; pp = 42; qq = 43; rr = 44; ss = 45
    return a + b + c + d + e + f + g + h + i + j + k + l + m + n + o + p + q + r + s + t + u + v + w + x + y + z
`
	writeFile(t, filepath.Join(dir, "app.py"), pySrc)

	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runScanner(t *testing.T, scannerBin, projectDir, projectKey string) []byte {
	t.Helper()
	cmd := exec.Command(scannerBin,
		"-project-dir", projectDir,
		"-project-key", projectKey,
		"-format", "json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scanner failed: %v\n%s", err, out)
	}

	reportPath := filepath.Join(projectDir, ".ollanta", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report.json: %v", err)
	}
	return data
}

func pushReport(t *testing.T, baseURL, token string, reportJSON []byte, idempotencyKey string) map[string]interface{} {
	t.Helper()

	var body bytes.Buffer
	gw := gzip.NewWriter(&body)
	if _, err := gw.Write(reportJSON); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/scans", &body)
	if err != nil {
		t.Fatalf("build push request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("push request failed: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("decode push response: %v\nbody: %s", err, string(b))
	}
	result["status_code"] = float64(resp.StatusCode)
	return result
}

func pollScanJob(t *testing.T, baseURL, token string, jobID int64, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("%s/api/v1/scan-jobs/%d", baseURL, jobID)

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var job map[string]interface{}
		if err := json.Unmarshal(b, &job); err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		status, _ := job["status"].(string)
		if status == "completed" || status == "failed" {
			t.Logf("scan job %d reached status %s", jobID, status)
			return job
		}
		t.Logf("scan job %d status: %s", jobID, status)
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("scan job %d did not reach terminal state within %v", jobID, timeout)
	return nil
}
