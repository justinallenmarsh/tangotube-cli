<p align="center">
  <img src="docs/images/tt.gif" width="348" alt="The TangoTube logo in a terminal: the couple in their embrace turn one giro, then the TangoTube wordmark walks in beneath them.">
</p>

# TangoTube. It's the command-line interface!

Argentine tango videos, in the terminal and in your coding agent's hands.

## Install

```bash
curl -fsSL https://tangotube.tv/install-cli | bash
```

On Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/justinallenmarsh/tangotube-cli/main/scripts/install.ps1 | iex
```

With mise:

```bash
mise use -g github:justinallenmarsh/tangotube-cli
```

From source, `go install`, and upgrading: [docs/install.md](docs/install.md).

## Getting started

```bash
tt search "di sarli noelia"
tt setup
```

The first finds Noelia Hurtado dancing to Carlos Di Sarli. The second teaches
your coding agent to do the same.

## Using the CLI

Search dancers, orchestras, songs… the way you would on the site:

```bash
tt search "di sarli facundo"
tt search --orchestra "di sarli" --year-from 1951 --year-to 1954
tt search --genre vals --dancer "noelia hurtado"
```

Find a step. Practice clips carry technique tags, so the search can too:

```bash
tt search "sacada" --dancer "noelia hurtado" --technique sacada
```

Look closer, then make a loop of the eight seconds you want:

```bash
tt video show 2ByaUQXeAeo
tt clip create 2ByaUQXeAeo --start 0:30 --end 0:42 --tag sacada
tt open 2ByaUQXeAeo
```

Browse the way the site does, and look before you search:

```bash
tt home                                   # the front page today
tt search --sort hidden-gems --kind class
tt resolve "di sarli noelia"              # what the words mean
tt facets --orchestra "di sarli"          # who and what a search holds
tt dancers --champion                     # and every other list
```

Dancers, couples, orchestras, songs, events and channels each have a page:

```bash
tt dancer show sebastian-achaval --timeline --tour
tt event show planetango
tt song versions todo-es-amor-fulvio-salamanca
tt performance show cPJ3MjWDUVY           # one dance, every camera on it
tt partners sebastian-achaval --depth 2   # who dances with whom
```

Keep what you find, as you would on the site:

```bash
tt like 2ByaUQXeAeo
tt playlist create "Di Sarli for Sunday" --video 2ByaUQXeAeo
tt follow dancer "noelia hurtado"
tt following --feed                       # what they posted this month
tt history                                # what you watched
tt notifications
```

Help name what TangoTube does not know yet:

```bash
tt queue --proposed                       # answers waiting for a yes
tt video show wCJInTctvnw --identity      # what we know, and how
tt suggest wCJInTctvnw --song la-mulateada-carlos-di-sarli --agree
tt report uv2Zqtyw3yM --kind not_tango
```

At a terminal the output is made for reading, and every id is a link: click
one to watch the performance on TangoTube. Piped, a command that returns data
writes JSON:

```bash
tt search "noelia" | jq '.data.videos[].id'
tt search "noelia" --jq '.videos[].id'      # no jq needed
```

Every command explains itself with `--help`. Searching needs no account;
clips, likes, playlists and the rest of your library do, so `tt auth login`
once.

## Coding agents

```bash
tt setup claude
tt setup codex
tt setup grok
```

That installs the tangotube skill, so "find me a sacada of Noelia to Di Sarli
and loop it" turns into the right three commands. Agents run with `--agent`:
JSON out, no prompts, no browser. Agents that speak MCP can use `tt mcp`.

## Docs

- [Install](docs/install.md)
- [CLI](docs/cli.md) — every command, the JSON envelope, exit codes
- [Agents](docs/agents.md) — Claude, Codex, Grok, and MCP

Something wrong? `tt doctor` checks the install, the connection, your sign-in,
and your agents.

When `tt` opens, the couple from the logo turn one giro. `TT_NO_ANIMATION=1`
keeps them still. `tt dance` lets them dance a whole tango; Ctrl-C says
gracias.

## Development

```bash
make          # check: gofmt, go vet, tests
make build    # ./bin/tt
make test
make smoke    # e2e/smoke.bats against recorded fixtures
make brand    # redraw docs/images/tt.gif from internal/brand
```

TangoTube is free, independent, and built by tango dancers.

## License

MIT. Copyright Justin Marsh. See [LICENSE](LICENSE). The TangoTube name and
logo are owned by Justin Marsh, all rights reserved, and are not covered by the
license; see [assets/brand](assets/brand).

TangoTube is not affiliated with YouTube. tt does not download video; playback
stays on YouTube.
