# Alex CLI Tool Windows Installation Script
# This script downloads and installs Alex CLI tool on Windows systems

param(
    [string]$Version = "",
    [string]$Repository = "cklxx/Alex-Code",
    [string]$InstallDir = "$env:LOCALAPPDATA\Alex",
    [switch]$Help
)

# 颜色输出函数
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    
    $colorMap = @{
        "Red" = "Red"
        "Green" = "Green"
        "Yellow" = "Yellow"
        "Blue" = "Blue"
        "White" = "White"
    }
    
    Write-Host $Message -ForegroundColor $colorMap[$Color]
}

function Write-Info {
    param([string]$Message)
    Write-ColorOutput "[INFO] $Message" "Blue"
}

function Write-Success {
    param([string]$Message)
    Write-ColorOutput "[SUCCESS] $Message" "Green"
}

function Write-Warning {
    param([string]$Message)
    Write-ColorOutput "[WARNING] $Message" "Yellow"
}

function Write-Error {
    param([string]$Message)
    Write-ColorOutput "[ERROR] $Message" "Red"
}

# 显示帮助信息
function Show-Help {
    Write-Host @"
Alex CLI Tool Windows Installation Script

USAGE:
    .\install.ps1 [OPTIONS]

OPTIONS:
    -Version VERSION      Install specific version (default: latest)
    -Repository REPO      GitHub repository (default: $Repository)
    -InstallDir DIR       Installation directory (default: $InstallDir)
    -Help                 Show this help message

EXAMPLES:
    .\install.ps1                                    # Install latest version
    .\install.ps1 -Version v1.0.0                   # Install specific version
    .\install.ps1 -InstallDir "C:\Program Files\Alex"  # Install to custom directory

"@
}

# 检测系统架构
function Get-SystemArchitecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { 
            Write-Error "Unsupported architecture: $arch"
            exit 1
        }
    }
}

# 获取最新版本
function Get-LatestVersion {
    Write-Info "Fetching latest version..."
    
    try {
        $apiUrl = "https://api.github.com/repos/$Repository/releases/latest"
        
        # 设置更robust的web请求参数
        $webClient = New-Object System.Net.WebClient
        $webClient.Headers.Add("User-Agent", "Alex-Installer/1.0")
        
        # 使用Invoke-RestMethod with timeout and retry
        $response = Invoke-RestMethod -Uri $apiUrl -Method Get -TimeoutSec 30 -ErrorAction Stop
        $latestVersion = $response.tag_name
        
        if (-not $latestVersion) {
            Write-Error "Failed to parse latest version from GitHub API response"
            Write-Info "You can specify a version manually with -Version parameter"
            exit 1
        }
        
        return $latestVersion
    }
    catch [System.Net.WebException] {
        Write-Error "Network error while fetching latest version: $($_.Exception.Message)"
        Write-Info "Please check your internet connection and try again"
        Write-Info "You can also specify a version manually with -Version parameter"
        exit 1
    }
    catch {
        Write-Error "Failed to fetch latest version: $($_.Exception.Message)"
        Write-Info "This might be due to GitHub API rate limiting"
        Write-Info "You can specify a version manually with -Version parameter"
        exit 1
    }
}

# 下载文件
function Download-File {
    param(
        [string]$Url,
        [string]$OutputPath
    )
    
    Write-Info "Downloading from: $Url"
    
    try {
        # 创建目录如果不存在
        $outputDir = Split-Path $OutputPath -Parent
        if (-not (Test-Path $outputDir)) {
            New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
        }
        
        # 下载文件
        Invoke-WebRequest -Uri $Url -OutFile $OutputPath -UseBasicParsing
        
        if (-not (Test-Path $OutputPath)) {
            Write-Error "Downloaded file not found: $OutputPath"
            return $false
        }
        
        return $true
    }
    catch {
        Write-Error "Download failed: $($_.Exception.Message)"
        return $false
    }
}

# 验证下载的文件
function Test-Binary {
    param([string]$BinaryPath)
    
    if (-not (Test-Path $BinaryPath)) {
        Write-Error "Downloaded binary not found: $BinaryPath"
        return $false
    }
    
    try {
        # 尝试运行 --version 检查
        $output = & $BinaryPath --version 2>$null
        if ($LASTEXITCODE -ne 0) {
            Write-Warning "Binary may not be working correctly (--version failed)"
            return $false
        }
        return $true
    }
    catch {
        Write-Warning "Binary may not be working correctly: $($_.Exception.Message)"
        return $false
    }
}

# 添加到PATH环境变量
function Add-ToPath {
    param([string]$Directory)
    
    # 获取当前用户PATH
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    
    # 检查是否已经在PATH中
    if ($currentPath -like "*$Directory*") {
        Write-Info "Directory already in PATH: $Directory"
        return
    }
    
    # 添加到PATH
    $newPath = if ($currentPath) { "$currentPath;$Directory" } else { $Directory }
    
    try {
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Success "Added to PATH: $Directory"
        Write-Info "Please restart your PowerShell session for changes to take effect"
    }
    catch {
        Write-Warning "Failed to add to PATH: $($_.Exception.Message)"
        Write-Info "You can manually add this directory to your PATH: $Directory"
    }
}

# 安装二进制文件
function Install-Binary {
    param(
        [string]$BinaryPath,
        [string]$TargetDir
    )
    
    # 确保安装目录存在
    if (-not (Test-Path $TargetDir)) {
        New-Item -ItemType Directory -Path $TargetDir -Force | Out-Null
        Write-Info "Created installation directory: $TargetDir"
    }
    
    # 复制二进制文件
    $targetPath = Join-Path $TargetDir "alex.exe"
    Copy-Item $BinaryPath $targetPath -Force
    
    Write-Success "Binary installed to: $targetPath"
    
    # 添加到PATH
    Add-ToPath $TargetDir
    
    # 验证安装
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "User") + ";" + [Environment]::GetEnvironmentVariable("Path", "Machine")
    
    if (Get-Command "alex" -ErrorAction SilentlyContinue) {
        Write-Success "Installation successful! You can now use 'alex'"
        try {
            $version = & alex --version 2>$null
            Write-Info "Installed version: $version"
        }
        catch {
            Write-Warning "Could not verify version"
        }
    }
    else {
        Write-Warning "Installation completed, but 'alex' is not found in PATH"
        Write-Info "You may need to restart your PowerShell session"
        Write-Info "Or use the full path: $targetPath"
    }
}

# 安装系统依赖
function Install-Dependencies {
    Write-Info "Installing system dependencies..."
    
    # 检测并安装ripgrep
    if (-not (Get-Command "rg" -ErrorAction SilentlyContinue)) {
        Write-Info "Installing ripgrep..."
        
        # 尝试使用winget (优先，因为是Windows内置)
        if (Get-Command "winget" -ErrorAction SilentlyContinue) {
            try {
                $wingetOutput = & winget install BurntSushi.ripgrep.GNU --accept-source-agreements --accept-package-agreements 2>&1
                if ($LASTEXITCODE -eq 0) {
                    Write-Success "ripgrep installed successfully via winget"
                } else {
                    Write-Warning "winget install failed with exit code $LASTEXITCODE"
                }
            }
            catch {
                Write-Warning "Failed to install ripgrep via winget: $($_.Exception.Message)"
            }
        }
        # 尝试使用Chocolatey
        elseif (Get-Command "choco" -ErrorAction SilentlyContinue) {
            try {
                $chocoOutput = & choco install ripgrep -y 2>&1
                if ($LASTEXITCODE -eq 0) {
                    Write-Success "ripgrep installed successfully via Chocolatey"
                } else {
                    Write-Warning "Chocolatey install failed with exit code $LASTEXITCODE"
                }
            }
            catch {
                Write-Warning "Failed to install ripgrep via Chocolatey: $($_.Exception.Message)"
            }
        }
        # 尝试使用Scoop
        elseif (Get-Command "scoop" -ErrorAction SilentlyContinue) {
            try {
                $scoopOutput = & scoop install ripgrep 2>&1
                if ($LASTEXITCODE -eq 0) {
                    Write-Success "ripgrep installed successfully via Scoop"
                } else {
                    Write-Warning "Scoop install failed with exit code $LASTEXITCODE"
                }
            }
            catch {
                Write-Warning "Failed to install ripgrep via Scoop: $($_.Exception.Message)"
            }
        }
        else {
            Write-Warning "No package manager found. Please install ripgrep manually from:"
            Write-Info "https://github.com/BurntSushi/ripgrep/releases"
        }
        
        # 验证安装
        if (Get-Command "rg" -ErrorAction SilentlyContinue) {
            Write-Success "ripgrep is now available"
        }
        else {
            Write-Warning "ripgrep installation may have failed or is not in PATH"
        }
    }
    else {
        Write-Info "ripgrep is already installed"
    }
}

# 主安装流程
function Install-Alex {
    Write-Info "Starting Alex CLI installation on Windows..."
    
    # 安装依赖
    Install-Dependencies
    
    # 检测系统架构
    $arch = Get-SystemArchitecture
    Write-Info "Detected architecture: $arch"
    
    # 获取版本
    if (-not $Version) {
        $Version = Get-LatestVersion
    }
    Write-Info "Installing version: $Version"
    
    # 构建下载URL
    $binaryName = "alex-windows-$arch.exe"
    $downloadUrl = "https://github.com/$Repository/releases/download/$Version/$binaryName"
    
    # 创建临时目录
    $tempDir = Join-Path $env:TEMP "alex-install"
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    # 下载二进制文件
    $tempBinaryPath = Join-Path $tempDir $binaryName
    if (-not (Download-File $downloadUrl $tempBinaryPath)) {
        Write-Error "Failed to download binary"
        exit 1
    }
    
    # 验证二进制文件
    if (-not (Test-Binary $tempBinaryPath)) {
        Write-Error "Binary verification failed"
        exit 1
    }
    
    # 安装到系统
    Install-Binary $tempBinaryPath $InstallDir
    
    # 清理临时文件
    Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    
    Write-Success "Alex CLI has been successfully installed!"
    Write-Info "Run 'alex --help' to get started"
}

# 主程序入口
if ($Help) {
    Show-Help
    exit 0
}

# 检查PowerShell版本
if ($PSVersionTable.PSVersion.Major -lt 5) {
    Write-Error "This script requires PowerShell 5.0 or later"
    exit 1
}

# 检查执行策略
$executionPolicy = Get-ExecutionPolicy
if ($executionPolicy -eq "Restricted") {
    Write-Warning "PowerShell execution policy is restricted"
    Write-Info "You may need to run: Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser"
    Write-Info "Or run this script with: powershell -ExecutionPolicy Bypass -File install.ps1"
}

# 运行安装
try {
    Install-Alex
}
catch {
    Write-Error "Installation failed: $($_.Exception.Message)"
    exit 1
} 