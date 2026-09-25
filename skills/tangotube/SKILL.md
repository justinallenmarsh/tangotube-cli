---
name: tangotube
description: Search and browse Argentine tango videos by dancer, couple, orchestra, song, event, year, style, and technique; list dancers, orchestras, events and Mundial champions; read lyrics; create practice loop clips; keep the user's likes, watch history, playlists, follows, practice list, saved searches and notifications; help name a video's dancers, song and event; and, for TangoTube operators with an admin token, read the pipelines, query the catalogue, change and undo records and pictures, work the review queues, land payloads, run and pause pipeline jobs, and put up announcements. Use when the user wants performances by a dancer or couple, music by an orchestra, a step reference, facts about the catalogue, a practice loop, or anything in their own TangoTube library. Do not scrape YouTube or tangotube.tv; use the tt CLI.
---

# TangoTube

A curated index of Argentine tango videos: who danced, to which recording,
which orchestra, where. A performance is one dance, perhaps filmed by several
cameras; a performance session is one couple's dances on one occasion. `tt`
reaches it all; every command answers in one JSON envelope.

## Install

```bash
curl -fsSL https://tangotube.tv/install-cli | bash
tt doctor --agent        # checks the binary, the API, the token and this skill
```

Reading needs no account; the user's own things need a token: they run `tt auth login`
(`--device` on a remote box) or set `TANGOTUBE_TOKEN`. Never ask for a token in the chat.
Without a shell, the same tools are the connector `https://tangotube.tv/mcp` (`/mcp/public`: search only).

## Rules

- Always pass `--agent`: the envelope, no colour, no prompts, no browser.
- Use `--jq` to pull out what you need: `tt search "di sarli" --agent --jq '.videos[].id'`.
- Follow `breadcrumbs`: the next commands that make sense, each runnable as written.
- Never download video or scrape tangotube.tv or YouTube. Hand over `watch_url`.
- Enumerate, don't guess: `tt dancers`, `tt orchestras`, `tt events`, … list
  real slugs; `tt resolve "words"` says what a phrase means before you search.
- Names work wherever a slug does, in flags and in the query. Check `data.filters`:
  a guessed name lists runners-up under `also`; rerun with the right slug.
- Give people the facts that come with it: `song` credits and `recorded_on`, where to
  listen (`song.links`), dancers' world `titles`, `other_angles` (other cameras on the same
  dance), `performance_session` (the couple's other dances that night). Lyrics:
  `tt song show SLUG --lyrics`; when `lyrics.en_source` is `machine`, say so.
- `--year` is when the **music was recorded** (`--year-from 1951 --year-to 1954`); `--uploaded`, the video.
- Times: `1:12`, `h:mm:ss` or seconds; clips run 2 s–5 min, tagged from `tt clip tags`. A YouTube link is an id.
- A write answers with the thing as it now is; doing it twice is doing it once ("Already …").
- Ask before anything that cannot be undone: `tt history clear --yes`, `tt playlist delete ID --yes`, `tt clip delete`.

## The envelope

```json
{"ok": true, "data": {}, "summary": "…", "breadcrumbs": ["tt …"]}
{"ok": false, "error": {"code": "not_found", "message": "…", "hint": "tt search …", "retryable": false}}
```

| exit | code | what to do |
|---|---|---|
| 0 | | read `data` |
| 1 | `usage` | fix the flags; `hint` shows a working call |
| 2 | `not_found` | list or resolve instead of guessing slugs |
| 3 | `auth` | ask the user to run `tt auth login` |
| 4 | `forbidden` | someone else's clip or playlist, a read-only token, or `tt admin` without an admin token; stop |
| 5 | `rate_limit` | wait, then retry once |
| 6 | `network` | run `tt doctor --agent` and report |
| 7 | `api` | report the message; do not retry in a loop |

## Commands

| Want | Run |
|---|---|
| Find performances | `tt search "QUERY" [--dancer --leader --follower --couple --orchestra --song --event --channel --genre --kind --year --year-from --year-to --uploaded --hd]` |
| Browse the catalogue | `tt search --sort trending\|popular\|newest\|oldest\|hidden-gems [--kind class]` |
| Find a step | `tt search "QUERY" --technique STEP [--style STYLE] [--dancer --orchestra --song]` |
| What a phrase means | `tt resolve "di sarli noelia"` |
| What a search holds | `tt facets [same filters as search]` — counts, plus recording years |
| Today's front page | `tt home` |
| Lists | `tt dancers [--champion --role]`, `tt couples`, `tt orchestras [--era]`, `tt songs [--orchestra --genre --decade]`, `tt events [--country]`, `tt channels`, `tt singers`, `tt champions [--year --category]` |
| One performance | `tt video show ID`; every video of that dance, and the couple's other dances that night: `tt performance show ID` |
| A page | `tt dancer show SLUG [--timeline --tour --repertoire]`, `tt couple show`, `tt orchestra show`, `tt song show SLUG [--lyrics]`, `tt event show SLUG [--year]`, `tt channel show ID` |
| Other recordings of a song | `tt song versions SLUG` |
| Who dances with whom | `tt partners SLUG [--depth 2]` — partners, then their partners |
| Who dances X best | `tt facets --orchestra "di sarli" --agent --jq '.facets.couple[:5]'`, then `tt couple show SLUG` |
| Practice clips | `tt clip list [--video ID] [--technique STEP] [--mine]`, `tt clip show ID`, `tt clip tags` |
| Make or remove a loop | `tt clip create VIDEO --start 1:12 --end 1:18 --tag sacada [--visibility public]`, `tt clip delete ID` |
| Edit a loop | `tt clip edit ID [--title --start --end --tag --add-tag --remove-tag --visibility]` |
| Likes | `tt like ID [--clip]`, `tt unlike ID`, `tt likes [QUERY] [--leader --follower --orchestra --event --year --sort recent\|newest\|popular] [--clips]` |
| Watch history | `tt history [QUERY] [same filters, --sort recent\|oldest\|popular]`, `tt history add VIDEO [--at 2024-05-02T21:30]`, `tt history rm VIDEO`, `tt history clear --yes` |
| Bring YouTube over | `tt import youtube PATH [--dry-run] [--no-history --no-likes --no-subscriptions] [--unknown FILE]`: the user's Google Takeout zip or folder, read locally; only ids and times are sent. Dry-run first and show the counts |
| Playlists | `tt playlist list`, `show ID`, `create TITLE [--visibility private\|unlisted\|public --video ID]`, `rename ID TITLE`, `edit ID`, `add ID VIDEO…`, `remove ID VIDEO`, `move ID VIDEO --to N`, `delete ID` |
| Follow | `tt follow dancer\|channel\|event NAME [--level personalized\|all_activity\|muted]`, `tt unfollow …`, `tt following [--kind dancers] [--feed]` |
| Practice list | `tt practice [--technique STEP]`, `tt practice add CLIP`, `tt practice rm CLIP` |
| Saved searches | `tt saved-search add NAME [QUERY] [search flags]`, `list`, `run NAME`, `rm NAME` |
| Notifications | `tt notifications [--unread]`, `tt notifications read` |
| A link; every command and flag | `tt open ID --agent --jq .url`; `tt commands --agent` |

`--technique` and `--style` search tagged practice clips (videos, each with its `clips`), and combine
only with `--dancer`, `--orchestra`, `--song`, `--genre`; any other filter is a `usage` error.

## Decision tree: "a sacada of Noelia to Di Sarli"

1. Is it a step? Yes, `sacada` is in `tt clip tags`. Search clips-first:

   ```bash
   tt search "sacada" --technique sacada --dancer "noelia hurtado" --orchestra "di sarli" --agent
   ```

2. `data.videos` is empty? Drop the narrowest filter and say so:
   - drop `--orchestra` → Noelia's sacadas to any orchestra;
   - or search performances without the step:
     `tt search --dancer "noelia hurtado" --orchestra "di sarli" --agent`,
     then offer to make the first sacada clip.
3. Pick a video. Read it in full: `tt video show VIDEO_ID --agent`.
   Search results' `clips[]` already hold the tagged `start_s`/`end_s`.
4. The user wants to practise it? Make a loop (needs a token):

   ```bash
   tt clip create VIDEO_ID --start 0:30 --end 0:42 --tag sacada --tag salon --agent
   ```

   Exit 3 means no token: ask the user to run `tt auth login`, then retry.
5. Hand it over: `tt open CLIP_ID --agent --jq .url` gives the link.

## Recipe: "make me a playlist of Noelia's Di Sarli"

```bash
tt search --dancer "noelia hurtado" --orchestra "di sarli" --sort popular --limit 8 --agent --jq '.videos[].id'
tt playlist create "Noelia to Di Sarli" --visibility unlisted --agent --jq .id
tt playlist add noelia-to-di-sarli ID1 ID2 ID3 --agent
```
Private unless the user says otherwise. Hand over `url` from the result.

## A real answer

`tt search "sacada" --technique sacada --dancer "noelia hurtado" --limit 1 --agent`, trimmed:

```json
{"ok": true, "data": {"videos": [{"id": "2ByaUQXeAeo", "watch_url": "https://tangotube.tv/watch?v=2ByaUQXeAeo",
   "dancers": [{"name": "Noelia Hurtado", "slug": "noelia-hurtado", "role": "follower"}, …],
   "song": {"title": "Cuando el amor muere", "recorded_on": "1941-08-02"},
   "clips": [{"id": "sacada-1-cuando-el-amor-muere", "start_s": 30, "end_s": 42, "tags": ["sacada", "salon"]}]}],
  "total": 3, "next_cursor": "2", "filters": {"dancer": {"slug": "noelia-hurtado", "name": "Noelia Hurtado"}}},
 "summary": "3 videos with practice clips · “sacada” · Noelia Hurtado",
 "breadcrumbs": ["tt open sacada-1-cuando-el-amor-muere", "tt video show 2ByaUQXeAeo"]}
```

## Helping TangoTube

Only when the user asks, and only what they said or confirmed: never put a guess
on a video, never sweep the queue on your own (it is rate-limited). Needs `write`.

1. What needs naming: `tt queue --proposed [--orchestra "di sarli"]`; one video:
   `tt video show ID --identity` (each fact settled, proposed, or unknown).
2. The easiest help is a yes: the user confirms a proposed song, run
   `tt suggest ID --song SLUG --agree`; somebody's waiting answer, `tt agree ID SUGGESTION`;
   the credited dancers are right, `tt confirm ID`.
3. Otherwise find the slug, `tt identify search song|dancer|event "words"`, then
   `tt suggest ID --dancer SLUG --role follower --note "how they know"`. A near miss
   is `usage` listing candidates; `--new` only for someone TangoTube lacks.
4. Always tell the user `data.outcome`: `applied` (on the video now) or `queued`
   (waiting for review, or for more people to agree).
5. Wrong? `tt report ID --kind not_tango|wrong_dancer|wrong_song|… [--dancer NAME]`.
   A step the tags lack: `tt clip tag-suggest CLIP STEP`.

## Operating TangoTube (operators only)

Needs an admin token: the operator runs `tt auth login --admin`; it lapses after 90 days.
- Look first: `tt admin dashboard|pipeline|jobs|intake|search-quality|channels|precision|desk VIDEO|coverage`.
- Anything else: `tt admin describe TYPE` for the columns, then `tt admin query "SELECT …"`
  (read-only, catalogue tables, 5 s, 200 rows). A refusal says why: fix the SQL, don't retry.
- Every change: `--dry-run` first and show the operator (it says what cannot be undone), then
  run it with `--note` saying why; `--yes` (MCP: `confirm: true`) only after they say yes. Verbs: `tt admin describe TYPE`.
- Records: `dancer edit|alias|merge`, `orchestra|singer|song|event|clip edit`, `song lyrics`,
  `channel review|activate|deactivate|reject-noise|sync`, `clip delete`, `championships load`,
  `video hide|unhide|feature|unfeature|label|stamp|reidentify|import`, `image add|list|primary|takedown|review`.
- Queues: `review list|accept|keep|reject`, `audio matches|verdict|rerun`, `suggestions`,
  `suggestion accept|reject`, `reports`, `report resolve|dismiss ID --note`, `tags pending`,
  `tag accept|reject|block|unblock`, `user show|trust|supporter EMAIL` (never roles or passwords),
  `performance recredit ID` (a credit on the video, not yet on the dancer page).
- Payloads: `payload export KIND -o FILE` → decide each row as its `decide` says →
  `payload import FILE --agent claude --dry-run`, then without; `payload status [--counts]|show ID`;
  `payload revert ID [--row REF] --yes`. A bad row refuses the file: fix that line, re-run.
- Pipelines: `job run NAME [--arg k=v]` (`--list`) states its cost; a full reindex, every channel
  or the audio sweep wants `--yes`, never unasked. `job retry|discard ID` (ids in `jobs`), `cron
  pause|resume KEY`, `rebuild STEP --yes` (`--list`); the site's banner, `announcement list|create|end`.
- `tt admin actions [--since 24h --mine --kinds]` is the log; `tt admin undo ID` puts one back,
  and refuses if the field changed since: report that, never work around it.
