# Install tt, the TangoTube CLI, on Windows.
#
#   irm https://raw.githubusercontent.com/justinallenmarsh/tangotube-cli/main/scripts/install.ps1 | iex
#
# TANGOTUBE_BIN_DIR    where tt.exe goes (default %LOCALAPPDATA%\Programs\tt)
# TANGOTUBE_VERSION    a release tag like v0.1.0 (default: the latest)
# TANGOTUBE_SKIP_SETUP set to skip installing the agent skill afterwards
#
# Windows PowerShell 5.1 or PowerShell 7. Nothing here needs an administrator.

& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue' # the progress bar makes downloads crawl in 5.1

  $repo = 'justinallenmarsh/tangotube-cli'
  $binDir = if ($env:TANGOTUBE_BIN_DIR) { $env:TANGOTUBE_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\tt' }
  $version = if ($env:TANGOTUBE_VERSION) { $env:TANGOTUBE_VERSION } else { 'latest' }

  function Say([string]$text) { [Console]::Error.WriteLine("  $text") }
  # throw, not exit: under `irm | iex` exit would close the person's window.
  function Die([string]$text) { throw "tt: $text" }

  # Windows PowerShell 5.1 still offers TLS 1.0 first, which GitHub refuses.
  [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

  # The machine's architecture, not this PowerShell's: an x64 PowerShell on an
  # Arm laptop should still get the arm64 build.
  $machine = $env:PROCESSOR_ARCHITEW6432
  if (-not $machine) { $machine = $env:PROCESSOR_ARCHITECTURE }
  switch ($machine) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { Die "tt ships for amd64 and arm64, not $machine" }
  }

  if ($version -eq 'latest') {
    $base = "https://github.com/$repo/releases/latest/download"
  } else {
    $base = "https://github.com/$repo/releases/download/$version"
  }
  $asset = "tt_windows_$arch.zip"

  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("tt-install-" + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Path $tmp | Out-Null
  try {
    try {
      Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
    } catch {
      Die "could not download $asset ($version): $($_.Exception.Message)"
    }
    try {
      Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')
    } catch {
      Die 'the release has no checksums.txt; not installing an unchecked binary'
    }

    $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match "^[0-9a-f]{64}\s+$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $line) { Die "checksums.txt does not list $asset" }
    $want = ($line -split '\s+')[0]
    $got = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $tmp $asset)).Hash.ToLowerInvariant()
    if ($got -ne $want) { Die "$asset does not match its checksum; not installing it" }

    Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath (Join-Path $tmp 'x') -Force
    Say "Downloaded tt ($version, windows/$arch), checksum verified."

    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    $target = Join-Path $binDir 'tt.exe'
    # A running tt.exe cannot be overwritten, but it can be renamed, which is
    # how tt upgrade replaces itself.
    $old = "$target.old"
    Remove-Item -Force -ErrorAction SilentlyContinue $old
    if (Test-Path $target) { Rename-Item -Path $target -NewName 'tt.exe.old' }
    Copy-Item -Path (Join-Path $tmp 'x\tt.exe') -Destination $target
    Say "Installed $target"
  } finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $tmp
  }

  # The user's PATH, not the machine's: no administrator needed. New windows
  # see it; this one is updated too.
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  $entries = @($userPath -split ';' | Where-Object { $_ })
  if ($entries -notcontains $binDir) {
    [Environment]::SetEnvironmentVariable('Path', (($entries + $binDir) -join ';'), 'User')
    Say "Added $binDir to your PATH. Open a new terminal for other windows to see it."
  }
  if (($env:Path -split ';') -notcontains $binDir) { $env:Path = "$env:Path;$binDir" }

  if (-not $env:TANGOTUBE_SKIP_SETUP -and [Environment]::UserInteractive -and -not [Console]::IsOutputRedirected) {
    try { & (Join-Path $binDir 'tt.exe') setup } catch { }
  }

  Say ''
  Say 'Next:  tt search "di sarli noelia"'
}
