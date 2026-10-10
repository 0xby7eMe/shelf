# Shelf installer for Windows.
#
#   irm https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.ps1 | iex
#
# Installs the latest release for your user (no administrator rights needed)
# into %LOCALAPPDATA%\Programs\Shelf, and adds Shelf to the Start menu.
#
# Options need the script as a script block:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.ps1))) -Version v1.2.3
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.ps1))) -Uninstall
#
#   -Version vX.Y.Z   install that release instead of the latest
#   -Uninstall        remove Shelf again (your settings are kept)

param(
	[string]$Version = 'latest',
	[switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # the progress bar makes downloads crawl in Windows PowerShell
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo = '0xby7eMe/shelf'
$Asset = 'shelf-windows-x86_64.exe'
$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\Shelf'
$Target = Join-Path $InstallDir 'shelf.exe'
$Programs = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'
$Shortcut = Join-Path $Programs 'Shelf.lnk'

function Write-Banner {
	Write-Host ''
	Write-Host '  shelf' -ForegroundColor Magenta -NoNewline
	Write-Host '   a minimal library for your games' -ForegroundColor DarkGray
	Write-Host ''
}

function Write-Step([string]$Text) { Write-Host "  $([char]0x2713) $Text" -ForegroundColor Green }
function Write-Note([string]$Text) { Write-Host "  $Text" -ForegroundColor DarkGray }
function Stop-With([string]$Text) {
	Write-Host "  x $Text" -ForegroundColor Red
	throw $Text
}

function Get-ReleaseUrl([string]$Name) {
	if ($Version -eq 'latest') { return "https://github.com/$Repo/releases/latest/download/$Name" }
	return "https://github.com/$Repo/releases/download/$Version/$Name"
}

function Remove-Shelf {
	Write-Banner
	Get-Process -Name shelf -ErrorAction SilentlyContinue |
		Where-Object { $_.Path -eq $Target } |
		Stop-Process -Force -ErrorAction SilentlyContinue
	Start-Sleep -Milliseconds 500
	foreach ($p in @($Target, "$Target.old", $Shortcut)) {
		Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue
	}
	if ((Test-Path $InstallDir) -and -not (Get-ChildItem $InstallDir)) { Remove-Item $InstallDir }
	# What Shelf itself added when asked to: game shortcuts and the shelf:// handler.
	Remove-Item -LiteralPath (Join-Path $Programs 'Shelf Games') -Recurse -Force -ErrorAction SilentlyContinue
	Remove-Item -LiteralPath 'HKCU:\Software\Classes\shelf' -Recurse -Force -ErrorAction SilentlyContinue
	Write-Step 'Removed Shelf'
	Write-Note "Your settings in $env:APPDATA\shelf and $env:LOCALAPPDATA\shelf were left alone"
	Write-Host ''
}

function Install-Shelf {
	Write-Banner
	if (-not [Environment]::Is64BitOperatingSystem) { Stop-With 'Shelf needs 64-bit Windows.' }
	if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
		Write-Note 'Windows on Arm runs the x86-64 build through emulation.'
	}

	$tmp = Join-Path ([IO.Path]::GetTempPath()) ("shelf-" + [guid]::NewGuid())
	New-Item -ItemType Directory -Path $tmp | Out-Null
	try {
		$exe = Join-Path $tmp $Asset
		try {
			Invoke-WebRequest -UseBasicParsing -Uri (Get-ReleaseUrl $Asset) -OutFile $exe
			Invoke-WebRequest -UseBasicParsing -Uri (Get-ReleaseUrl "$Asset.sha256") -OutFile "$exe.sha256"
		} catch {
			Stop-With "Couldn't download Shelf ($Version): $($_.Exception.Message)"
		}
		Write-Step 'Downloaded Shelf'

		$want = ((Get-Content "$exe.sha256" -Raw).Trim() -split '\s+')[0].ToLower()
		$got = (Get-FileHash -Algorithm SHA256 -LiteralPath $exe).Hash.ToLower()
		if (-not $want -or $want -ne $got) { Stop-With "The download doesn't match its checksum, so it was thrown away." }
		Write-Step 'Checksum matches'

		New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
		# A running Shelf can't be overwritten, but it can step aside; it is
		# deleted the next time Shelf starts.
		if (Test-Path $Target) {
			Remove-Item -LiteralPath "$Target.old" -Force -ErrorAction SilentlyContinue
			Move-Item -LiteralPath $Target -Destination "$Target.old" -Force
		}
		Move-Item -LiteralPath $exe -Destination $Target -Force
		Unblock-File -LiteralPath $Target -ErrorAction SilentlyContinue
		Write-Step "Installed to $Target"

		$shell = New-Object -ComObject WScript.Shell
		$link = $shell.CreateShortcut($Shortcut)
		$link.TargetPath = $Target
		$link.WorkingDirectory = $InstallDir
		$link.IconLocation = "$Target,0"
		$link.Description = 'A minimal library for your Steam, Epic, GOG and Ubisoft games'
		$link.Save()
		Write-Step 'Added Shelf to the Start menu'
	} finally {
		Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
	}

	Write-Host ''
	Write-Host '  Start Shelf from the Start menu. Run this again to update.' -ForegroundColor Cyan
	Write-Host ''
}

if ($Uninstall) { Remove-Shelf } else { Install-Shelf }
