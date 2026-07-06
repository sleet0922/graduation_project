param(
    [string]$SshUser = "root",
    [string]$SshHost = "server.gelsomino.cn",
    [string]$RemoteDir = "/root/server/graduation_project"
)

$ErrorActionPreference = "Stop"

function Get-OpenSshTool {
    param([Parameter(Mandatory = $true)][string]$Name)

    $candidates = @(
        (Join-Path $env:WINDIR "Sysnative\OpenSSH\$Name.exe"),
        (Join-Path $env:WINDIR "System32\OpenSSH\$Name.exe")
    )

    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            return $candidate
        }
    }

    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    throw "Cannot find $Name.exe. Please install Windows OpenSSH client."
}

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$Script
    )

    Write-Host "`n$Name" -ForegroundColor Cyan
    & $Script
}

$ProjectRoot = Split-Path -Parent $PSCommandPath
$GoOutBin = Join-Path $ProjectRoot "main"
$ConfigFile = Join-Path $ProjectRoot "configs\config.yaml"
$Ssh = Get-OpenSshTool -Name "ssh"
$Scp = Get-OpenSshTool -Name "scp"
$Remote = "$SshUser@$SshHost"

Set-Location $ProjectRoot

try {
    Invoke-Step "Step1: Cross compile Linux amd64 binary" {
        $env:CGO_ENABLED = "0"
        $env:GOAMD64 = "v3"
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"

        go build `
            -trimpath `
            -ldflags="-s -w -extldflags=-static -compressdwarf=false" `
            -gcflags="all=-l=4 -B -C -d=checkptr=0" `
            -o $GoOutBin `
            ./cmd/api/main.go
    }

    Invoke-Step "Step2: Upload files" {
        & $Ssh $Remote "mkdir -p '$RemoteDir/configs' '$RemoteDir/logs'"
        & $Scp $GoOutBin "${Remote}:$RemoteDir/main"
        & $Scp $ConfigFile "${Remote}:$RemoteDir/configs/config.yaml"
    }

    Invoke-Step "Step3: Restart remote service" {
        $remoteCommand = "cd '$RemoteDir'; sudo fuser -k 8081/tcp 2>/dev/null || true; chmod +x ./main; nohup ./main > logs/app.log 2>&1 &"
        & $Ssh $Remote $remoteCommand
    }

    Write-Host "`nDeploy finished!" -ForegroundColor Green
}
finally {
    Invoke-Step "Step4: Clean local binary" {
        Remove-Item -Force $GoOutBin -ErrorAction SilentlyContinue
    }
}
