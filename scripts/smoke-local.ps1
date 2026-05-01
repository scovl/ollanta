[CmdletBinding()]
param(
    [int]$BackendPort = 18080,
    [string]$ConfigPath,
    [string]$ScannerToken = "ollanta-dev-scanner-token",
    [int]$DependencyTimeoutSeconds = 60,
    [int]$ServerTimeoutSeconds = 60,
    [switch]$KeepTempProject
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $repoRoot "config.toml.example"
}
$ConfigPath = [System.IO.Path]::GetFullPath($ConfigPath)

$mingwBin = "C:\msys64\mingw64\bin"
$success = $false
$tempProjectDir = $null
$projectKey = $null
$serverJob = $null
$selectedBackendPort = $BackendPort
$serverLogPath = Join-Path ([System.IO.Path]::GetTempPath()) ("ollanta-smoke-server-" + [System.Guid]::NewGuid().ToString("N") + ".log")

function Require-Command {
    param([string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "required command '$Name' was not found on PATH"
    }
}

function Test-TcpPort {
    param(
        [string]$HostName,
        [int]$Port
    )

    $client = New-Object System.Net.Sockets.TcpClient
    $asyncResult = $null
    try {
        $asyncResult = $client.BeginConnect($HostName, $Port, $null, $null)
        if (-not $asyncResult.AsyncWaitHandle.WaitOne(1000)) {
            return $false
        }
        $client.EndConnect($asyncResult)
        return $true
    } catch {
        return $false
    } finally {
        if ($asyncResult -ne $null) {
            $asyncResult.AsyncWaitHandle.Close()
        }
        $client.Close()
    }
}

function Wait-TcpPort {
    param(
        [string]$HostName,
        [int]$Port,
        [int]$TimeoutSeconds,
        [string]$Name
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-TcpPort -HostName $HostName -Port $Port) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "$Name did not become reachable on $HostName`:$Port within $TimeoutSeconds seconds"
}

function Get-AvailablePort {
    param(
        [int]$PreferredPort,
        [int]$Attempts = 20
    )

    for ($offset = 0; $offset -lt $Attempts; $offset++) {
        $candidate = $PreferredPort + $offset
        if (-not (Test-TcpPort -HostName "127.0.0.1" -Port $candidate)) {
            return $candidate
        }
    }

    throw "no free smoke-test backend port found starting at $PreferredPort"
}

function Get-JobFailureMessage {
    param([string]$LogPath)

    if (Test-Path $LogPath) {
        return (Get-Content $LogPath -Raw)
    }
    return "<no server log captured>"
}

function Wait-HttpReady {
    param(
        [string]$Url,
        [int]$TimeoutSeconds,
        [System.Management.Automation.Job]$Job,
        [string]$LogPath
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
                return
            }
        } catch {
        }

        $state = (Get-Job -Id $Job.Id).State
        if ($state -in @("Completed", "Failed", "Stopped")) {
            throw "local ollantaweb exited before becoming ready. Server log:`n$(Get-JobFailureMessage -LogPath $LogPath)"
        }
        Start-Sleep -Milliseconds 500
    }

    throw "local ollantaweb did not become ready at $Url within $TimeoutSeconds seconds. Server log:`n$(Get-JobFailureMessage -LogPath $LogPath)"
}

function Ensure-ScannerEnvironment {
    if (Test-Path $mingwBin) {
        $pathParts = @($env:PATH -split ";")
        if ($pathParts -notcontains $mingwBin) {
            $env:PATH = "$mingwBin;$env:PATH"
        }
    }
    $env:CGO_ENABLED = "1"
    Require-Command -Name "go"
    Require-Command -Name "git"
    if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
        throw "scanner smoke validation requires gcc on PATH; expected MinGW at $mingwBin"
    }
}

function New-SmokeProject {
    $stamp = Get-Date -Format "yyyyMMddHHmmss"
    $script:projectKey = "smoke-config-toml-$stamp"
    $script:tempProjectDir = Join-Path ([System.IO.Path]::GetTempPath()) ("ollanta-smoke-" + $stamp)

    New-Item -ItemType Directory -Path $script:tempProjectDir | Out-Null
    Set-Content -Path (Join-Path $script:tempProjectDir "go.mod") -Value @(
        "module smoke",
        "",
        "go 1.21"
    )
    Set-Content -Path (Join-Path $script:tempProjectDir "main.go") -Value @(
        "package main",
        "",
        'import "fmt"',
        "",
        "func main() {",
        '    fmt.Println("smoke")',
        "}"
    )

    & git -C $script:tempProjectDir init -b main | Out-Null
    & git -C $script:tempProjectDir add . | Out-Null
    & git -C $script:tempProjectDir -c user.name="Ollanta Smoke" -c user.email="smoke@local.test" commit -m "smoke" | Out-Null
}

function Start-SmokeServer {
    param(
        [string]$RepoRoot,
        [string]$ResolvedConfigPath,
        [int]$ListenPort,
        [string]$LogPath
    )

    return Start-Job -ScriptBlock {
        param($RepoRoot, $ResolvedConfigPath, $ListenPort, $LogPath)
        $ErrorActionPreference = "Stop"
        Set-Location $RepoRoot
        $env:OLLANTA_CONFIG_FILE = $ResolvedConfigPath
        $env:OLLANTA_ADDR = ":$ListenPort"
        go run github.com/scovl/ollanta/ollantaweb/cmd/ollantaweb *>&1 | Tee-Object -FilePath $LogPath
    } -ArgumentList $RepoRoot, $ResolvedConfigPath, $ListenPort, $LogPath
}

try {
    if (-not (Test-Path $ConfigPath)) {
        throw "config file not found: $ConfigPath"
    }

    Require-Command -Name "docker"
    $null = & docker info 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "docker engine is unavailable; start Docker Desktop or the Docker daemon first"
    }

    Write-Host "Starting PostgreSQL and ZincSearch via Docker Compose..."
    & docker compose --profile server up -d postgres zincsearch | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "failed to start postgres and zincsearch via docker compose"
    }

    Wait-TcpPort -HostName "127.0.0.1" -Port 5432 -TimeoutSeconds $DependencyTimeoutSeconds -Name "PostgreSQL"
    Wait-TcpPort -HostName "127.0.0.1" -Port 4080 -TimeoutSeconds $DependencyTimeoutSeconds -Name "ZincSearch"

    $selectedBackendPort = Get-AvailablePort -PreferredPort $BackendPort
    if ($selectedBackendPort -ne $BackendPort) {
        Write-Warning "backend port $BackendPort is in use; using $selectedBackendPort for smoke validation"
    }

    Write-Host "Starting local ollantaweb on port $selectedBackendPort..."
    $serverJob = Start-SmokeServer -RepoRoot $repoRoot -ResolvedConfigPath $ConfigPath -ListenPort $selectedBackendPort -LogPath $serverLogPath
    Wait-HttpReady -Url "http://127.0.0.1:$selectedBackendPort/readyz" -TimeoutSeconds $ServerTimeoutSeconds -Job $serverJob -LogPath $serverLogPath

    Ensure-ScannerEnvironment
    New-SmokeProject

    Write-Host "Running scanner against temporary project $projectKey..."
    $env:OLLANTA_CONFIG_FILE = $ConfigPath
    & go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta `
        -project-dir $tempProjectDir `
        -project-key $projectKey `
        -format summary `
        -local-ui=false `
        -server "http://127.0.0.1:$selectedBackendPort" `
        -server-token $ScannerToken `
        -server-wait `
        -server-wait-timeout 2m `
        -server-wait-poll 1s
    if ($LASTEXITCODE -ne 0) {
        throw "scanner smoke validation failed"
    }

    $headers = @{ Authorization = "Bearer $ScannerToken" }
    $readyResponse = Invoke-WebRequest -Uri "http://127.0.0.1:$selectedBackendPort/readyz" -UseBasicParsing -Headers $headers
    if ($readyResponse.StatusCode -ne 200) {
        throw "readyz returned $($readyResponse.StatusCode), expected 200"
    }

    $scanResponse = Invoke-WebRequest -Uri "http://127.0.0.1:$selectedBackendPort/api/v1/projects/$projectKey/scans/latest" -UseBasicParsing -Headers $headers
    if ($scanResponse.StatusCode -ne 200) {
        throw "latest scan endpoint returned $($scanResponse.StatusCode), expected 200"
    }

    $success = $true
    Write-Host "Smoke validation passed for project key $projectKey on port $selectedBackendPort"
} finally {
    if ($serverJob -ne $null) {
        try {
            Stop-Job -Id $serverJob.Id -ErrorAction SilentlyContinue | Out-Null
        } finally {
            Remove-Job -Id $serverJob.Id -Force -ErrorAction SilentlyContinue | Out-Null
        }
    }

    if ($success -and -not $KeepTempProject -and $tempProjectDir -and (Test-Path $tempProjectDir)) {
        Remove-Item -Path $tempProjectDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    if ($success -and (Test-Path $serverLogPath)) {
        Remove-Item -Path $serverLogPath -Force -ErrorAction SilentlyContinue
    } elseif (-not $success) {
        if ($tempProjectDir) {
            Write-Warning "smoke project preserved at $tempProjectDir"
        }
        if (Test-Path $serverLogPath) {
            Write-Warning "server log preserved at $serverLogPath"
        }
    }
}