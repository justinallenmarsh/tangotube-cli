# Installing tt

## The one-liner

```bash
curl -fsSL https://tangotube.tv/install-cli | bash
```

The script finds your OS and architecture (macOS or Linux, amd64 or arm64),
downloads the latest release from GitHub, checks it against the release's
`checksums.txt`, and puts `tt` in `~/.local/bin`. At a terminal it then runs
`tt setup` so any coding agent it finds gets the skill.

Choose another directory:

```bash
curl -fsSL https://tangotube.tv/install-cli | TANGOTUBE_BIN_DIR=/usr/local/bin bash
```

Pin a version:

```bash
curl -fsSL https://tangotube.tv/install-cli | TANGOTUBE_VERSION=v0.1.0 bash
```

The same script lives at [`scripts/install.sh`](../scripts/install.sh). Read it
before you pipe it to a shell; it is short.

## Windows

In PowerShell (Windows PowerShell 5.1 or PowerShell 7), no administrator
needed:

```powershell
irm https://raw.githubusercontent.com/justinallenmarsh/tangotube-cli/main/scripts/install.ps1 | iex
```

The script picks amd64 or arm64, downloads `tt_windows_<arch>.zip` from the
latest release, checks it against the release's `checksums.txt`, puts
`tt.exe` in `%LOCALAPPDATA%\Programs\tt`, and adds that directory to your
user `PATH`. Open a new terminal afterwards so it sees the change.
`TANGOTUBE_BIN_DIR` and `TANGOTUBE_VERSION` work as they do for the shell
script:

```powershell
$env:TANGOTUBE_VERSION = 'v0.1.0'; irm https://raw.githubusercontent.com/justinallenmarsh/tangotube-cli/main/scripts/install.ps1 | iex
```

The script lives at [`scripts/install.ps1`](../scripts/install.ps1).

On Windows the token lives in the Credential Manager, or in
`%AppData%\tangotube\token` with `TANGOTUBE_NO_KEYRING=1`. Colour and links
need Windows Terminal or a console that reads escape sequences; the old
console gets plain text. Pictures of the logo show in WezTerm and are off
elsewhere, and `TT_IMAGES` overrides the guess as it does everywhere.

## mise

```bash
mise use -g github:justinallenmarsh/tangotube-cli
```

## go install

```bash
go install github.com/justinallenmarsh/tangotube-cli/cmd/tt@latest
```

Go 1.22 or newer. The binary lands in `$(go env GOPATH)/bin`.

## From source

```bash
git clone https://github.com/justinallenmarsh/tangotube-cli
cd tangotube-cli
make build        # ./bin/tt
```

If no release exists yet, the install script does this for you when Go is
installed.

## PATH

The installer tells you when `~/.local/bin` is not on your `PATH`. Add it:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc    # or ~/.bashrc
```

`tt doctor` says whether the `tt` it finds on your `PATH` is the one you just
installed.

## Upgrade

```bash
tt upgrade
```

That runs the install script again into the directory `tt` already lives in
(on Windows, `install.ps1`, which sets the running `tt.exe` aside as
`tt.exe.old` and puts the new one in its place).
With mise: `mise upgrade`. With Go: `go install …@latest` again.

## Uninstall

```bash
tt auth logout
rm ~/.local/bin/tt
rm -r ~/.config/tangotube ~/.claude/skills/tangotube ~/.codex/skills/tangotube ~/.grok/skills/tangotube
```

On Windows, delete `%LOCALAPPDATA%\Programs\tt` and `%AppData%\tangotube`,
and remove the directory from your user `PATH` in Settings → System → About →
Advanced system settings → Environment Variables.

Revoke the token itself in Settings → Tokens on tangotube.tv.
