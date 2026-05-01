package ollantarules_test

import (
	"testing"
	"testing/fstest"

	ollantarules "github.com/scovl/ollanta/ollantarules"
)

func TestLoadMeta_HappyPath(t *testing.T) {
	fsys := fstest.MapFS{
		"go_no-large-functions.json": &fstest.MapFile{
			Data: []byte(`{"key":"go:no-large-functions","name":"No Large Functions","language":"go","type":"code_smell","severity":"major"}`),
		},
		"go_todo-comment.json": &fstest.MapFile{
			Data: []byte(`{"key":"go:todo-comment","name":"TODO Comment","language":"go","type":"code_smell","severity":"info"}`),
		},
	}

	meta, err := ollantarules.LoadMeta(fsys, "*.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meta) != 2 {
		t.Errorf("expected 2 entries, got %d", len(meta))
	}
	m, ok := meta["go:no-large-functions"]
	if !ok {
		t.Fatal("expected key go:no-large-functions")
	}
	if m.Name != "No Large Functions" {
		t.Errorf("Name: got %q", m.Name)
	}
	if m.Language != "go" {
		t.Errorf("Language: got %q", m.Language)
	}
}

func TestLoadMeta_MalformedJSON(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.json": &fstest.MapFile{Data: []byte(`{not valid json`)},
	}
	_, err := ollantarules.LoadMeta(fsys, "*.json")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestLoadMeta_EmptyKey(t *testing.T) {
	fsys := fstest.MapFS{
		"nokey.json": &fstest.MapFile{
			Data: []byte(`{"key":"","name":"Test","language":"go","type":"code_smell","severity":"info"}`),
		},
	}
	_, err := ollantarules.LoadMeta(fsys, "*.json")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestLoadMeta_DuplicateKey(t *testing.T) {
	fsys := fstest.MapFS{
		"a.json": &fstest.MapFile{Data: []byte(`{"key":"go:dup","name":"A","language":"go","type":"code_smell","severity":"info"}`)},
		"b.json": &fstest.MapFile{Data: []byte(`{"key":"go:dup","name":"B","language":"go","type":"code_smell","severity":"info"}`)},
	}
	_, err := ollantarules.LoadMeta(fsys, "*.json")
	if err == nil {
		t.Error("expected error for duplicate key")
	}
}

func TestLoadMeta_EmptyFS(t *testing.T) {
	meta, err := ollantarules.LoadMeta(fstest.MapFS{}, "*.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meta) != 0 {
		t.Errorf("expected empty map, got %d entries", len(meta))
	}
}

func TestLoadMeta_BadGlob(t *testing.T) {
	_, err := ollantarules.LoadMeta(fstest.MapFS{}, "[invalid")
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}

func TestWithMeta(t *testing.T) {
	meta := ollantarules.RuleMeta{Key: "go:test", Name: "Test", Language: "go"}
	r := ollantarules.Rule{}.WithMeta(meta)
	if r.Meta.Key != "go:test" {
		t.Errorf("WithMeta Key: got %q", r.Meta.Key)
	}
	if r.Meta.Name != "Test" {
		t.Errorf("WithMeta Name: got %q", r.Meta.Name)
	}
}

var _ = fstest.MapFS{} // ensure fstest is used
