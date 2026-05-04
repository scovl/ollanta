param(
    [string]$ServerUrl = $(if ($env:OLLANTA_SERVER_URL) { $env:OLLANTA_SERVER_URL } else { "http://localhost:8080" }),
    [string]$AdminLogin = $(if ($env:OLLANTA_ADMIN_LOGIN) { $env:OLLANTA_ADMIN_LOGIN } else { "admin" }),
    [string]$AdminPassword = $env:OLLANTA_ADMIN_PASSWORD,
    [string]$ProjectKey = $("smoke-custom-rules-{0}" -f [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()),
    [switch]$KeepArtifacts
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($AdminPassword)) {
    throw "Set OLLANTA_ADMIN_PASSWORD or pass -AdminPassword before running this smoke test."
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$apiBase = $ServerUrl.TrimEnd("/") + "/api/v1"
$token = ""
$createdRuleId = 0
$createdProfileId = 0
$createdProject = $false
$projectDir = Join-Path ([System.IO.Path]::GetTempPath()) $ProjectKey

function ConvertTo-JsonBody($Value) {
    return $Value | ConvertTo-Json -Depth 30 -Compress
}

function Invoke-Api($Method, $Path, $Body = $null) {
    $headers = @{}
    if (-not [string]::IsNullOrWhiteSpace($script:token)) {
        $headers["Authorization"] = "Bearer $script:token"
    }
    $request = @{
        Uri         = $script:apiBase + $Path
        Method      = $Method
        Headers     = $headers
        ErrorAction = "Stop"
    }
    if ($null -ne $Body) {
        $request["ContentType"] = "application/json"
        $request["Body"] = ConvertTo-JsonBody $Body
    }
    return Invoke-RestMethod @request
}

try {
    Invoke-RestMethod -Uri ($ServerUrl.TrimEnd("/") + "/readyz") -Method Get -ErrorAction Stop | Out-Null

    $login = Invoke-RestMethod -Uri ($apiBase + "/auth/login") -Method Post -ContentType "application/json" -Body (ConvertTo-JsonBody @{ login = $AdminLogin; password = $AdminPassword })
    $token = $login.access_token
    if ([string]::IsNullOrWhiteSpace($token)) {
        throw "Login did not return an access token."
    }

    $namespace = $ProjectKey
    $marker = "OLLANTA_CUSTOM_RULE_SMOKE_MARKER"
    $pack = @{
        version = 1
        pack = @{
            name = $ProjectKey
            namespace = $namespace
            description = "Smoke test custom rule pack"
        }
        rules = @(@{
            key = "no-smoke-marker"
            name = "No smoke marker"
            language = "go"
            type = "code_smell"
            severity = "major"
            engine = "text"
            engine_config = @{ pattern = $marker }
            message = "Remove the smoke marker."
            examples = @(
                @{
                    name = "compliant"
                    code = "package main`n`nfunc main() {`n    println(`"ok`")`n}`n"
                    compliant = $true
                },
                @{
                    name = "noncompliant"
                    code = "package main`n`nfunc main() {`n    println(`"$marker`")`n}`n"
                    compliant = $false
                    want_line = 4
                }
            )
        })
    }

    $created = Invoke-Api Post "/custom-rules" $pack
    $rule = $created.items[0]
    $createdRuleId = [int64]$rule.id
    $ruleKey = [string]$rule.key
    if ($createdRuleId -le 0 -or [string]::IsNullOrWhiteSpace($ruleKey)) {
        throw "Custom rule creation did not return an id and rule key."
    }

    $validated = Invoke-Api Post ("/custom-rules/{0}/validate" -f $createdRuleId)
    if ($validated.validation_status -ne "passed") {
        throw "Custom rule validation status was '$($validated.validation_status)'."
    }

    $preview = Invoke-Api Post ("/custom-rules/{0}/preview" -f $createdRuleId) @{ file_path = "main.go"; source = "package main`nfunc main() { println(`"$marker`") }`n" }
    if ([int]$preview.match_count -lt 1) {
        throw "Custom rule preview did not find the expected marker."
    }

    $published = Invoke-Api Post ("/custom-rules/{0}/publish" -f $createdRuleId)
    if ($published.lifecycle -ne "published") {
        throw "Custom rule lifecycle after publish was '$($published.lifecycle)'."
    }

    $project = Invoke-Api Post "/projects" @{ key = $ProjectKey; name = $ProjectKey; main_branch = "main"; tags = @("smoke") }
    if ([int64]$project.id -le 0) {
        throw "Project creation did not return an id."
    }
    $createdProject = $true

    $profile = Invoke-Api Post "/profiles/" @{ name = ("{0} profile" -f $ProjectKey); language = "go" }
    $createdProfileId = [int64]$profile.id
    if ($createdProfileId -le 0) {
        throw "Profile creation did not return an id."
    }
    Invoke-Api Post ("/profiles/{0}/rules" -f $createdProfileId) @{ rule_key = $ruleKey; severity = "info"; params = @{} } | Out-Null
    Invoke-Api Post ("/projects/{0}/profiles" -f [uri]::EscapeDataString($ProjectKey)) @{ language = "go"; profile_id = $createdProfileId } | Out-Null

    New-Item -ItemType Directory -Force -Path $projectDir | Out-Null
    Set-Content -Path (Join-Path $projectDir "main.go") -Encoding UTF8 -Value "package main`n`nfunc main() {`n    println(`"$marker`")`n}`n"

    Push-Location (Join-Path $repoRoot "ollantascanner")
    try {
        & go run ./cmd/ollanta `
            -project-dir $projectDir `
            -project-key $ProjectKey `
            -sources ./... `
            -format json `
            -server $ServerUrl `
            -server-token $token `
            -server-wait `
            -server-wait-timeout 2m `
            -server-wait-poll 1s `
            -profile-source server `
            -profile-strict
        if ($LASTEXITCODE -ne 0) {
            throw "Scanner exited with code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }

    $reportPath = Join-Path $projectDir ".ollanta\report.json"
    $report = Get-Content -Raw -Path $reportPath | ConvertFrom-Json
    $matchingIssue = @($report.issues | Where-Object { $_.rule_key -eq $ruleKey })
    if ($matchingIssue.Count -lt 1) {
        throw "Scanner report did not contain the published custom rule issue."
    }
    if ($report.scanner_options.custom_rules.sources -notcontains "server") {
        throw "Scanner report did not record the server custom rule source."
    }

    $issuesPath = "/issues?project_id=$($project.id)&rule_key=$([uri]::EscapeDataString($ruleKey))&limit=20"
    $serverIssues = Invoke-Api Get $issuesPath
    if ([int]$serverIssues.total -lt 1) {
        throw "Server issues API did not return the custom rule issue."
    }

    [pscustomobject]@{
        status = "passed"
        server = $ServerUrl
        project_key = $ProjectKey
        rule_key = $ruleKey
        scanner_issues = $matchingIssue.Count
        server_issues = $serverIssues.total
    } | ConvertTo-Json -Depth 5
}
finally {
    if (-not $KeepArtifacts) {
        if ($createdRuleId -gt 0 -and -not [string]::IsNullOrWhiteSpace($token)) {
            try { Invoke-Api Post ("/custom-rules/{0}/disable" -f $createdRuleId) | Out-Null } catch { Write-Warning $_ }
        }
        if ($createdProfileId -gt 0 -and -not [string]::IsNullOrWhiteSpace($token)) {
            try { Invoke-Api Delete ("/profiles/{0}" -f $createdProfileId) | Out-Null } catch { Write-Warning $_ }
        }
        if ($createdProject -and -not [string]::IsNullOrWhiteSpace($token)) {
            try { Invoke-Api Delete ("/projects/{0}" -f [uri]::EscapeDataString($ProjectKey)) | Out-Null } catch { Write-Warning $_ }
        }
        if (Test-Path $projectDir) {
            Remove-Item -Recurse -Force $projectDir
        }
    }
}
