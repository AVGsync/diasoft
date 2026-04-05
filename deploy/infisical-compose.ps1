[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ComposeArgs
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Resolve-Path (Join-Path $ScriptDir "..")

$InfisicalEnvFile = if ($env:INFISICAL_ENV_FILE) { $env:INFISICAL_ENV_FILE } else { Join-Path $RootDir ".env.infisical" }
$LocalEnvFile = if ($env:LOCAL_ENV_FILE) { $env:LOCAL_ENV_FILE } else { Join-Path $RootDir ".env.machine" }

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "docker is required."
}

function Import-DotEnvFile {
    param([string]$Path)

    if (-not (Test-Path $Path)) {
        return
    }

    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ([string]::IsNullOrWhiteSpace($line) -or $line.StartsWith("#")) {
            return
        }

        $separatorIndex = $line.IndexOf("=")
        if ($separatorIndex -lt 1) {
            return
        }

        $name = $line.Substring(0, $separatorIndex).Trim()
        $value = $line.Substring($separatorIndex + 1)

        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }

        [Environment]::SetEnvironmentVariable($name, $value)
    }
}

$TempEnvFile = Join-Path ([System.IO.Path]::GetTempPath()) ("diasoft-infisical-{0}.env" -f ([guid]::NewGuid()))

try {
    if (Test-Path $LocalEnvFile) {
        Copy-Item $LocalEnvFile $TempEnvFile -Force
    }
    else {
        New-Item -ItemType File -Path $TempEnvFile -Force | Out-Null
    }

    Add-Content -Path $TempEnvFile -Value ""
    Import-DotEnvFile -Path $InfisicalEnvFile
    if (Test-Path $InfisicalEnvFile) {
        Add-Content -Path $TempEnvFile -Value (Get-Content $InfisicalEnvFile)
    }

    Write-Host "Running docker compose with merged env files..."
    Push-Location $RootDir
    try {
        & docker compose --env-file $TempEnvFile @ComposeArgs
        exit $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
}
finally {
    if (Test-Path $TempEnvFile) {
        Remove-Item $TempEnvFile -Force
    }
}
