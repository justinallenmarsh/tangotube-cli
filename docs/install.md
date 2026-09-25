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

That runs the install script again into the directory `tt` already lives in.
With mise: `mise upgrade`. With Go: `go install …@latest` again.

## Uninstall

```bash
tt auth logout
rm ~/.local/bin/tt
rm -r ~/.config/tangotube ~/.claude/skills/tangotube ~/.codex/skills/tangotube ~/.grok/skills/tangotube
```

Revoke the token itself in Settings → Tokens on tangotube.tv.
