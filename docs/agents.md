# tt for coding agents

Your agent already has a shell. `tt` gives it the tango catalog: search by
dancer, orchestra, song, year, style, and step; read a performance; make a
practice loop for the person it is helping.

## Setup

```bash
curl -fsSL https://tangotube.tv/install-cli | bash
tt setup
```

`tt setup` copies the tangotube skill to every agent it finds:

| Agent | Skill goes to |
|---|---|
| Claude Code | `~/.claude/skills/tangotube/SKILL.md` |
| Codex | `~/.codex/skills/tangotube/SKILL.md` |
| Grok | `~/.grok/skills/tangotube/SKILL.md` |
| This project | `.claude/skills/tangotube/` or `.agents/skills/tangotube/`, when those directories exist |

Name one to install it before the agent has run here: `tt setup claude`.
`tt skill` prints the skill; `tt doctor` says which agents have it and whether
it is current. Run `tt setup` again after `tt upgrade`.

## What the skill does

It tells the agent when to reach for `tt` — a dancer or couple, an
orchestra's music, a step reference, a practice loop — and how:

- always `--agent`, so output is one JSON envelope with no prompts;
- follow `breadcrumbs`; read `error.hint` on failure; act on the exit code;
- `--technique` for steps, `--year` for the recording year;
- `search → video show → clip create → open`;
- never download video, never scrape tangotube.tv or YouTube.

It carries a decision tree for "a sacada of Noelia to Di Sarli" and a real
envelope, so the agent knows the shape before its first call.

It also says how to help name videos, and when not to: only what the person
said or confirmed, a yes to a proposal before a fresh claim, and always the
outcome — applied, or waiting for review.

For operators it adds `tt admin`: look before changing anything, ask the
catalogue with `tt admin describe` and a read-only `tt admin query`, preview
every change with `--dry-run`, give a `--note`, pass `--yes` only after the
operator says yes, and when `tt admin undo` refuses because a field moved on,
report it rather than work around it.

## Tokens for agents

Search needs no token. Clips do. Three ways, from most to least hands-off:

1. **The person signs tt in once:** `tt auth login` on their machine. The
   agent's `tt` finds the token in the keyring or `~/.config/tangotube/token`.
2. **The agent runs somewhere else** (SSH, a container, a cloud IDE):
   `tt auth login --device` prints a code; the person types it at
   tangotube.tv/device.
3. **CI or a sandbox:** make a token in Settings → Tokens and set
   `TANGOTUBE_TOKEN`.

`--agent` never opens a browser and never prompts. A write without a token
exits 3 with `tt auth login` as the hint; the agent should ask the person, not
retry.

## MCP

Agents that speak the Model Context Protocol rather than shell:

```bash
claude mcp add tangotube -- tt mcp
```

`tt mcp` serves the catalogue — `search`, `facets`, `resolve`, `home`,
`catalogue_list`, `video_show`, `performance_show`, `entity_show`,
`song_versions`, `partners`, `clip_list`, `clip_create`, `clip_tags` — and the signed-in person's library: `like`,
`likes`, `history`, `history_edit`, `playlists`, `playlist_edit`, `follow`,
`following`, `practice`, `saved_searches`, `notifications`, and `clip_edit` —
and helping name videos: `queue`, `identify_search`, `suggest`, `confirm`,
`agree`, `report`, `tag_suggest` (`video_show` takes `identity`).
The tools that change things take an `action` (`playlist_edit` with `create`,
`add`, `move`…). Each tool returns the same envelope the CLI prints, and uses
the token `tt` already has; clearing history needs `confirm: true`.

With an operator's admin token (`tt auth login --admin`), `tt mcp` also serves
`admin_report`, `admin_coverage`, `admin_desk`, `admin_describe`,
`admin_query`, `admin_actions`, `admin_undo`, `admin_video` and the rest of
`tt admin`. It asks the API at start-up whether the token is an operator's, and
lists them only then; the API checks again on every call. An operator tool
that changes the catalogue only previews until it is called with
`confirm: true`, which only the person may give.

The same tools, with the same names and arguments, are served remotely at
`https://tangotube.tv/mcp` (which signs you in) for Claude on a phone or
claude.ai, and without an account at `https://tangotube.tv/mcp/public`: see
[Use TangoTube from Claude on your phone](claude-phone.md).

`tt import youtube` has no MCP tool on purpose. It reads a Google Takeout
export from the user's disk, and a remote MCP client has no disk to read.
An agent with a shell runs the command; with MCP alone, the user runs it.

## No chatbot, on purpose

There is no concierge inside tangotube.tv. The site is for looking; the agent
you already use is for asking. `tt` is the bridge between them: a small,
predictable tool your agent can call, not another assistant to talk to.
