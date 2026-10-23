# Dmanga Windows Installer Script
# Usage: irm https://raw.githubusercontent.com/bubz727/Dmanga/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

Write-Host "=== Dmanga Installer (Windows) ===" -ForegroundColor Cyan

# 1. Determine Architecture
$arch = if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "Unsupported 32-bit Windows operating system."
    exit 1
}

$binaryName = "Dmanga_windows_${arch}.exe"
Write-Host "[*] Detected architecture: $arch ($binaryName)" -ForegroundColor Gray

# 2. Query Latest Release from GitHub API
$apiUrl = "https://api.github.com/repos/bubz727/Dmanga/releases/latest"
Write-Host "[*] Fetching latest release information..." -ForegroundColor Gray

try {
    $release = Invoke-RestMethod -Uri $apiUrl -Headers @{ "User-Agent" = "Dmanga-Installer" }
} catch {
    Write-Error "Failed to fetch latest release from GitHub: $_"
    exit 1
}

$tag = $release.tag_name
Write-Host "[+] Latest version: $tag" -ForegroundColor Green

# 3. Locate Target Asset Download URLs
$binaryAsset = $release.assets | Where-Object { $_.name -eq $binaryName }
if (-not $binaryAsset) {
    Write-Error "Binary asset '$binaryName' not found in release $tag."
    exit 1
}

$checksumAsset = $release.assets | Where-Object { $_.name -match "SHA256SUMS\.txt|checksums\.txt" }

# 4. Target Directory
$installDir = Join-Path $env:LOCALAPPDATA "Programs\Dmanga"
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

$destPath = Join-Path $installDir "Dmanga.exe"
$tempPath = Join-Path $installDir "Dmanga.exe.tmp"

# 5. Download Binary
Write-Host "[*] Downloading $binaryName..." -ForegroundColor Gray
Invoke-WebRequest -Uri $binaryAsset.browser_download_url -OutFile $tempPath -UseBasicParsing

# 6. Verify Checksum if present
if ($checksumAsset) {
    Write-Host "[*] Verifying SHA256 checksum..." -ForegroundColor Gray
    try {
        $checksumsText = (Invoke-RestMethod -Uri $checksumAsset.browser_download_url -Headers @{ "User-Agent" = "Dmanga-Installer" })
        $expectedHash = ""
        foreach ($line in ($checksumsText -split "`r?`n")) {
            $parts = $line.Trim() -split "\s+"
            if ($parts.Count -ge 2 -and $parts[1] -match $binaryName) {
                $expectedHash = $parts[0]
                break
            }
        }
        if ($expectedHash) {
            $actualHash = (Get-FileHash -Path $tempPath -Algorithm SHA256).Hash.ToLower()
            if ($actualHash -ne $expectedHash.ToLower()) {
                Remove-Item -Path $tempPath -Force -ErrorAction SilentlyContinue
                Write-Error "Checksum verification failed! Expected: $expectedHash, Got: $actualHash"
                exit 1
            }
            Write-Host "[OK] Checksum verified successfully." -ForegroundColor Green
        }
    } catch {
        Write-Warning "Could not verify checksum, continuing installation..."
    }
}

# 7. Safe Install / Replace
if (Test-Path $destPath) {
    $oldPath = Join-Path $installDir "Dmanga.exe.old"
    Remove-Item -Path $oldPath -Force -ErrorAction SilentlyContinue
    Move-Item -Path $destPath -Destination $oldPath -Force -ErrorAction SilentlyContinue
}
Move-Item -Path $tempPath -Destination $destPath -Force

# 8. Add to User PATH if not present
$userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -split ";" -notcontains $installDir) {
    Write-Host "[*] Adding '$installDir' to your User PATH..." -ForegroundColor Cyan
    $newPath = "$userPath;$installDir".TrimStart(";")
    [System.Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$env:Path;$installDir"
}

Write-Host ""
Write-Host "[OK] Dmanga $tag was successfully installed to:" -ForegroundColor Green
Write-Host "     $destPath" -ForegroundColor White
Write-Host ""
Write-Host "You can now run 'Dmanga' directly in any terminal window!" -ForegroundColor Yellow
