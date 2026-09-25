# Working on tt

tt is the TangoTube CLI: a Go binary that talks to the `/api/v1` JSON API on
tangotube.tv. It must stand alone — nothing here may import or assume the
TangoTube web app, because this directory becomes its own public
repository.

## Loop

```bash
make            # gofmt check, go vet, go test ./... (unit + e2e on fixtures)
make build      # ./bin/tt
make smoke      # bats e2e/smoke.bats (mise installs bats; see mise.toml)
```

Tests never touch the network or a database. `e2e/fixtures/*.json` are
recorded envelopes served by `e2e/fixtures.Handler`; `go run
./e2e/fixtureserver` serves them on 127.0.0.1:4599 for poking by hand:

```bash
TANGOTUBE_API_URL=http://127.0.0.1:4599 ./bin/tt search "di sarli"
```

Against a local TangoTube: `TANGOTUBE_DEV=1 ./bin/tt …` (localhost:3000), or
`--api-url`.

## Layout

| Path | What |
|---|---|
| `cmd/tt` | `main`: runs the tree, turns errors into exit codes |
| `internal/commands` | one file per verb family; `app.go` holds shared wiring |
| `internal/api` | HTTP client; every response is an `output.Envelope` |
| `internal/output` | envelope, modes (human/JSON/quiet), tables, palette, `--jq` |
| `internal/auth` | token precedence, keyring/file store, PKCE and device flows |
| `skills/tangotube/SKILL.md` | the agent skill, embedded by `skills/embed.go` |
| `scripts/install.sh` | what `https://tangotube.tv/install-cli` serves |

## Rules

- Read `STYLE.md`. Help text is product copy.
- A new API field or route: add a fixture first, then the command, then docs
  (`docs/cli.md`, and `SKILL.md` if agents need it).
- Exit codes and the envelope are a public contract. Don't change them.
- `SKILL.md` stays under 180 lines.

## GitHub listing

- Repository: `justinallenmarsh/tangotube-cli`
- Description: `TangoTube CLI and agent skill`
- Topics: `tango`, `cli`, `agent-skills`, `golang`
- License: MIT (`LICENSE`)
- Releases: GoReleaser (`.goreleaser.yaml`) — darwin/linux × amd64/arm64 and
  `checksums.txt`. Tag `vX.Y.Z` and run `goreleaser release --clean`.
