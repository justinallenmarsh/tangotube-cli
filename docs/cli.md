# The tt CLI

```bash
tt search "di sarli facundo"
tt search "sacada" --technique sacada --dancer "noelia hurtado"
tt facets --orchestra "di sarli"
tt dancers --champion
tt video show 2ByaUQXeAeo
tt clip create 2ByaUQXeAeo --start 0:30 --end 0:42 --tag sacada
tt playlist add di-sarli-for-sunday 2ByaUQXeAeo
tt following --feed
tt open sacada-1-cuando-el-amor-muere
```

At a terminal, output is made for reading: a summary line, a table, and the
`next:` commands worth running. Piped, or with `--json`, every command that
returns data writes one JSON envelope. `tt commands --json` describes every
command and flag below, for a program.

## Search

```bash
tt search [QUERY] [flags]
```

Type what you would type into the search box: *Search dancers, orchestras,
songs…*

```bash
tt search "di sarli facundo"
tt search --dancer "noelia hurtado" --orchestra "di sarli"
tt search --orchestra "di sarli" --year-from 1951 --year-to 1954
tt search --genre milonga --limit 12
tt search --leader "carlitos" --follower "noelia hurtado"
tt search --sort hidden-gems --kind class
tt search "sacada" --technique sacada --style salon
```

| Flag | Means |
|---|---|
| `--dancer NAME` | A dancer, by name or slug. |
| `--leader NAME`, `--follower NAME` | A dancer in that role. |
| `--channel NAME` | A YouTube channel, by title or id (`tt channels`). |
| `--kind KIND` | `performance`, `class`, `workshop`, `interview`, `competition`, … |
| `--uploaded YEAR` | The year the **video went up** on YouTube. |
| `--hd` | Only HD uploads. |
| `--sort ORDER` | `browse` (the default), `trending`, `popular`, `newest`, `oldest`, `hidden-gems`. |
| `--orchestra NAME` | An orchestra, by name or slug. |
| `--song TITLE` | A recording, by title or slug. |
| `--event NAME` | A festival or milonga. |
| `--couple NAMES` | A partnership, by both names or its slug. |
| `--genre GENRE` | `tango`, `vals`, or `milonga`. |
| `--year YEAR` | The year the **music was recorded**. |
| `--year-from YEAR`, `--year-to YEAR` | A range of recording years. |
| `--technique STEP` | A step tagged on practice clips (`tt clip tags`). |
| `--style STYLE` | `milonguero`, `salon`, `nuevo`, or `stage`, tagged on practice clips. |
| `--cursor CURSOR` | The next page: `data.next_cursor` from the last one. |

Names resolve to slugs on the server and come back in `data.filters`: "di sarli",
"disarli", and "D'Arienzo" all work. Names typed into the query are read as
filters too, so `tt search "di sarli facundo"` searches Carlos Di Sarli and a
Facundo. When a name could be several people, `data.filters` names the one
chosen and lists the rest under `also`, and at a terminal tt says so above the
table. If that reading finds nothing, the words are searched as typed.

A sort, a kind, a channel or `--hd` alone is enough: `tt search --sort newest`
browses the whole catalogue the way the site's sections do.

Search counts performances, not uploads: twelve cameras on one dance are one
result. It pages as far as the first 10,000.

A video has no style or technique of its own; dancers tag the practice clips
they make. So `--technique` and `--style` search clips and return the videos
they come from, each carrying `clips`. They combine with `--dancer`,
`--orchestra`, `--song`, `--genre`, and each other. Combined with `--event`,
`--couple`, or a year flag they are a `usage` error, not a silently dropped
filter; so are `--sort`, `--kind`, `--channel`, `--hd` and `--uploaded`.

## Before searching: resolve, facets, home

```bash
tt resolve "di sarli noelia"             # what the words mean, before searching
tt facets --orchestra "di sarli"         # who and what a search holds, counted
tt home                                  # the front page of tangotube.tv today
```

`tt resolve` names what a typed phrase matches — dancers, couples,
orchestras, songs, events, channels — and how `tt search` would read it as
filters, with the reading's own matches first. When nothing matches it offers
the spelling it thinks you meant.

`tt facets` takes the same query and filters as `tt search` and counts what is
in the result: kinds of video, genres, leaders, followers, couples, songs,
events, channels, upload years, and — drawn as a bar per decade — the years
the music was recorded. Each facet is labelled with the flag that narrows to
it. Counts are performances. `--limit` is how many of each.

`tt home` is the site's front page: trending, new uploads, the featured
dancer, orchestra, song, event and channel, conversations, classes, hidden
gems and the Mundial, three videos each at a terminal (six in JSON), each
section with the `tt search` that shows all of it.

## Lists

```bash
tt dancers [--role leader|follower] [--champion] [--sort popular|name|recent|followers|partners]
tt couples [--min-videos N] [--sort popular|name|recent|most-videos]
tt orchestras [--era golden-age|contemporary] [--sort popular|name|most-songs|most-videos]
tt songs [--orchestra NAME] [--genre GENRE] [--decade 1940] [--letter L] [--sort popular|name|newest|oldest]
tt events [--country NAME] [--continent NAME] [--category KIND] [--sort …]
tt channels [--sort popular|name|recent|followers]
tt singers [--orchestra NAME] [--sort popular|name]
tt champions [--year YEAR] [--category pista|salon|escenario]
```

The site's index pages, a page at a time, most watched first. A bare word (or
`--q`) narrows by name. Each row starts with the slug to paste into the next
command, linked to its page; counts line up on the right. `--cursor` takes the
next page from `data.next_cursor`, and the breadcrumb spells it out.

## Show

```bash
tt video show uGwRPRusbC0                           # the YouTube id, as in /watch?v=ID
tt video show uGwRPRusbC0 --identity                # what we know about it, and how
tt video show "https://youtu.be/uGwRPRusbC0"        # a pasted link works too
tt dancer show noelia-hurtado
tt couple show carlitos-espinoza-noelia-hurtado
tt orchestra show carlos-di-sarli
tt song show volver-a-sonar-carlos-di-sarli
tt song show volver-a-sonar-carlos-di-sarli --lyrics  # the words, Spanish and English
tt song versions todo-es-amor-fulvio-salamanca        # the same song, other recordings
tt dancer show sebastian-achaval --timeline --tour --repertoire
tt event show planetango [--year 2019]
tt channel show UCCoOxQMnmwZ-jhezbLfSUgQ              # the YouTube channel id
tt performance show cPJ3MjWDUVY                       # one dance, every video of it
tt partners noelia-hurtado [--depth 2]                # who dances with whom
```

A video reads the way you take in a performance: who danced (with any world
titles they hold), the recording (singer, composer, lyricist, the day it was
recorded), the festival and its dates, the video (length, upload, views on
YouTube), other cameras on the same dance, and where to listen: Spotify,
YouTube Music, El Recodo. With links on, the listen line is three words to
click; with `TT_NO_LINKS=1` each address is printed in full. When the video is
one of a couple's dances on one occasion, `session` lists them all, this one
marked, each a link.

A song shows the same credits and listening links, how many performances are
danced to it, and those performances. `--lyrics` prints the words instead, in
Spanish and then in English where there is a translation, wrapped to 72
columns with the verses as written; `--json --lyrics` adds
`lyrics: {es, en, en_source}`. When `en_source` is `machine` the English is a
machine translation, and the heading says so. `tt song versions` lists the
other recordings of the same composition.

`tt performance show` takes the YouTube id of any video of a dance (or the
`performance` id `tt video show` returns) and lists every video of it: a
Mundial final is often filmed from three sides of the floor, and a channel
sometimes posts one dance twice. Each video is a row of channel, length and
views, the one asked about marked. Above them the song with its credits and
the occasion; below, when the couple danced two to six dances that occasion,
all of them in order, each with its own count of videos.

`tt partners` is the network the site draws on /explore: each partner with
the videos they share with the dancer, most first, and their slug for the
next command. `--depth 2` adds, under each of the top partners, who else
they dance with, and names the dancers several of them share. `--json` gives
the graph whole: `nodes` with `ring` (0 the dancer, 1 a partner, 2 beyond)
and `edges` weighted by videos.

A dancer shows world titles, the years they have been filmed, partners, the
orchestras they dance to most, and their performances. A couple shows its
years and orchestras. An orchestra shows the years its danced recordings come
from, the singers dancers choose most with it, and its most-danced songs.
Every one ends with its TangoTube page.

`tt dancer show` takes the dancer page's profile too: `--timeline` (videos
per year, by kind, and each partnership's years), `--tour` (the cities, and
the newest performances on the road with how many dances each), and
`--repertoire` (the songs danced, year by year). An event shows its editions,
the couples, dancers, orchestras and songs filmed there most, and similar
events. A channel shows its trending and newest videos.

## Clips

*Create short practice clips from any video. Loop movements, tag techniques,
and build your personal library.*

```bash
tt clip tags                                  # the vocabulary
tt clip list --video 2ByaUQXeAeo
tt clip list --technique sacada --dancer "noelia hurtado"
tt clip list --mine
tt clip show sacada-1-cuando-el-amor-muere
tt clip create 2ByaUQXeAeo --start 0:30 --end 0:42 --tag sacada --tag salon
tt clip create 2ByaUQXeAeo --start 72 --end 78 --tag boleo --title "Boleo out of the ocho" --visibility public
tt clip edit sacada-at-1-12 --title "Sacada into the cross" --end 1:20 --add-tag boleo
tt clip delete sacada-at-1-12
```

Times are `m:ss`, `h:mm:ss`, or seconds. A clip runs two seconds to five
minutes and ends inside the video. Sending the same clip twice gives back the
one already made, so an agent can retry safely. Clips are private unless you say `--visibility unlisted` or `public`.
Tags come from `tt clip tags`; write `media luna` as `media_luna` (tt does it
for you). Creating, editing and deleting need a token with write access.
`tt clip edit` changes only what you say and keeps the clip's id; `--tag`
replaces the tags, `--add-tag` and `--remove-tag` change them.

## Your library

Everything you keep on the site, from the terminal. It all needs a token
(`tt auth login`): reading takes `read`, changing takes `write`. Each change
answers with the thing as it now is, and doing it twice is doing it once where
the site works that way — liking, following, saving for practice, adding to a
playlist.

```bash
tt like 2ByaUQXeAeo                          # a video; --clip for a practice clip
tt unlike 2ByaUQXeAeo
tt likes --orchestra "di sarli" --sort popular
tt likes --clips

tt history                                   # what you watched, newest first
tt history pugliese --sort popular           # a word in the title, and the same filters as likes
tt history add 2ByaUQXeAeo --at 2024-05-02T21:30
tt history rm 2ByaUQXeAeo
tt history clear --yes                       # there is no undo

tt playlist create "Di Sarli for Sunday" --visibility unlisted --video 2ByaUQXeAeo
tt playlist add di-sarli-for-sunday uGwRPRusbC0 n07s2yjCs-E
tt playlist move di-sarli-for-sunday n07s2yjCs-E --to 1
tt playlist show di-sarli-for-sunday         # anyone's public or unlisted playlist, too
tt playlist rename di-sarli-for-sunday "Sunday night"
tt playlist edit di-sarli-for-sunday --visibility public
tt playlist remove di-sarli-for-sunday uGwRPRusbC0
tt playlist list
tt playlist delete di-sarli-for-sunday

tt follow dancer "noelia hurtado"
tt follow event planetango --level all_activity   # personalized, all_activity, or muted
tt follow channel UCtdgMR0bmogczrZNpPaO66Q --level muted
tt unfollow dancer "noelia hurtado"
tt following                                 # who you follow
tt following --feed                          # their new videos from the last 30 days

tt practice                                  # clips saved to loop
tt practice add sacada-1-cuando-el-amor-muere
tt practice rm sacada-1-cuando-el-amor-muere

tt saved-search add "Di Sarli vals" --orchestra "di sarli" --genre vals
tt saved-search list                         # each with the tt search that runs it
tt saved-search run "Di Sarli vals"
tt saved-search rm "Di Sarli vals"

tt notifications                             # the bell, newest first
tt notifications --unread
tt notifications read                        # mark them all read
```

Likes and history filter the way the site's pages do: `--leader`,
`--follower`, `--orchestra`, `--genre`, `--event`, `--channel`, `--kind`,
`--year` (the year it was filmed), by name or slug. Your first follow turns on
the weekly email of new videos, as it does on the site; Settings turns it off.

### Bring your YouTube over

```bash
tt import youtube ~/Downloads/takeout-20260924T101500Z-001.zip --dry-run
tt import youtube ~/Downloads/takeout-20260924T101500Z-001.zip --unknown not-on-tangotube.txt
tt import youtube ~/Downloads/Takeout --no-history      # likes and follows only
```

`tt import youtube` brings your watch history, liked videos and subscriptions
over from YouTube: the watches, likes and follows TangoTube has a video or
channel for. YouTube's API has not given out watch history since 2016, so it
starts from Google Takeout. At [takeout.google.com](https://takeout.google.com),
deselect all, pick **YouTube and YouTube Music**, keep *history*, *playlists* and
*subscriptions*, and choose JSON for history. PATH is the zip Google sends, the
folder it unzips to, or one file from it.

What it reads:

| Part | Files | Notes |
|---|---|---|
| Watches | `history/watch-history.json` or `watch-history.html` | YouTube Music plays, ads and removed videos are left out. HTML dates are read in English and day-first formats; others arrive dated now. |
| Likes | `playlists/Liked videos.csv` (older exports) or `Liked videos-videos.csv` (newer) | Found in any language Takeout names it in (`Vídeos que me gustan`, `Mag ich`, …). |
| Follows | `subscriptions/subscriptions.csv` | Any CSV of `UC…` channel ids outside the playlists folder. |

The export is read on your machine. The only things sent are YouTube video ids
and the times you watched or liked them, plus channel ids, in requests of
2,000 each. Watches and likes keep YouTube's dates. Importing again adds only
what is new, and never moves a watch earlier. Imported follows do not sign you
up for the weekly email. `--unknown FILE` writes the YouTube links TangoTube
does not have, one per line. Imports have their own rate limit of 60 requests
an hour per token, separate from the rest of the API.

## Helping TangoTube

TangoTube names most of its videos by itself, and not all of them well. You
can say who danced, to which song, and where, as the watch page asks.

```bash
tt queue --proposed                                   # answers waiting for a yes
tt queue --fact song --orchestra "di sarli"           # Di Sarli videos without a song
tt video show wCJInTctvnw --identity                  # settled, proposed, or unknown
tt suggest wCJInTctvnw --song la-mulateada-carlos-di-sarli --agree
tt identify search dancer guspero                     # the slug to name somebody with
tt suggest k-shh3fWESA --dancer daiana-guspero --role follower --note "in the title"
tt suggest k-shh3fWESA --batch changes.json           # several at once, all or nothing
tt confirm k-shh3fWESA                                # the credited dancers are right
tt agree k-shh3fWESA 4812                             # somebody's waiting answer is right
tt report uv2Zqtyw3yM --kind not_tango                # no account needed
tt clip tag-suggest parallel-cross-sacada-turn-linear-exit "cross system"
```

The site's rules decide what happens, and every answer says which:

| | when |
|---|---|
| applied | you agreed with what a machine proposed (`--agree`), or you have standing on the site |
| sent for review | you are new here, or you are correcting something already settled |

An answer waiting for review also applies once three people agree with it
(`tt agree`). Names are matched strictly: `Diana Guspero` is not
`daiana-guspero`, so a near miss lists the candidates rather than guessing,
and someone TangoTube does not have yet needs `--new`. Sending the same answer
twice sends it once. A report weighs 1 without an account, 1.5 signed in and
3 from a trusted contributor; 3 takes the fact down until somebody looks.
Contributions are limited to 60 an hour per token, anonymous reports to 20 an
hour.

A batch file is a JSON list:

```json
[{"fact": "kind", "op": "replace", "kind": "performance"},
 {"fact": "dancers", "op": "add", "dancer": "daiana-guspero"}]
```

`op` is `add`, `replace`, `remove` (a report about that credit, with
`appearance_id`) or `withdraw` (your own waiting answer, with `suggestion_id`).

## Operating TangoTube

For the people who run TangoTube. Every `tt admin` command needs an admin
token, which only an operator's account can approve and which lapses after
90 days:

```bash
tt auth login --admin          # or --admin --device on a remote box
tt auth status                 # says admin, and when it expires
```

A dancer's token, or a write token, gets exit 4 (`forbidden`) with
`tt auth login --admin` as the hint. So does an admin token whose account has
since stopped being an operator: the role is checked on every request.

### Look

```bash
tt admin dashboard             # catalogue, imports, people, search, audio
tt admin pipeline              # imports by day, channel syncs, the audio pipeline
tt admin intake                # this week's arrivals and how they were classified
tt admin search-quality        # clicks, rank, queries that found nothing
tt admin channels --limit 20   # the channel scorecard
tt admin coverage              # the heartbeat: what is named, every alarm
tt admin coverage --pile conflict
tt admin desk EJv04w-mZaM      # what every source said about one video
tt admin precision             # how right each matching method has been
tt admin jobs                  # queues, the schedule (paused or when next), what failed
```

Each returns the object its `/admin` page reads, so the two cannot disagree.
The dashboard names people but never shows their email addresses.

### Ask

```bash
tt admin describe              # the kinds of record
tt admin describe dancer       # table, columns, enum values, links, verbs
tt admin query "SELECT dance_form, count(*) FROM videos GROUP BY 1 ORDER BY 2 DESC"
tt admin query - < question.sql --limit 1000
```

`tt admin query` takes one `SELECT` and nothing else. It reads catalogue
tables only (videos, dancers, couples, songs, orchestras, events, channels,
performances and the pipelines' own tables) and never accounts, tokens,
sessions, OAuth grants, likes, watches, playlists, notifications or
payloads, nor `pg_catalog` or `information_schema`. It calls aggregate,
window, text, number, date, array and JSON functions only. It runs read-only,
stops after 5 seconds, and returns 200 rows (`--limit` up to 2,000), saying
when there were more. A refusal is exit 1 naming the reason.

### Change, and undo

```bash
tt admin video hide EJv04w-mZaM --note "reupload of 2ByaUQXeAeo" --dry-run
tt admin video hide EJv04w-mZaM --note "reupload of 2ByaUQXeAeo"
tt admin actions --since 24h   # what changed, by whom, and whether it can be undone
tt admin actions --kind payload --cursor 2
tt admin actions --kinds       # every verb the log holds, and which undo
tt admin action 812
tt admin undo 812 --dry-run
tt admin undo 812
```

Every change takes `--dry-run`, which answers with what would change and
changes nothing. Anything destructive or hard to reverse needs `--yes`.
Every change is recorded with its before and after, and your `--note`.

`tt admin undo` is a compare-and-swap: it puts the change back only while
every field it touched still holds the value it left. If anything has
changed one since (another operator, a suggestion, a nightly job), it refuses
and names the field, the value the change left, and the value now. Undoing
another operator's change needs `--yes`. An action is undone once.

### Records

```bash
tt admin dancer edit noelia-hurtado --bio "Born in Buenos Aires." --role follower --dry-run
tt admin dancer alias add noelia-hurtado "Noe Hurtado" --replay
tt admin dancer alias list noelia-hurtado
tt admin dancer alias rm noelia-hurtado "Noe Hurtado"
tt admin dancer merge bailaron-neolia-hurtado into noelia-hurtado --dry-run
tt admin orchestra edit carlos-di-sarli --bio "El Señor del Tango."
tt admin singer edit alberto-echague --name "Alberto Echagüe"
tt admin song edit volver-a-sonar-carlos-di-sarli --composer "Carlos Di Sarli" --recorded 1941-01-01
tt admin song lyrics volver-a-sonar-carlos-di-sarli --es letra.txt --en lyrics.txt
tt admin event edit planetango --city Moscow --start 2026-07-01 --end 2026-07-05 --lat 55.75 --lng 37.61
tt admin channel review|activate|deactivate|reject-noise|sync UCCoOxQMnmwZ-jhezbLfSUgQ
tt admin video feature|unfeature|reidentify EJv04w-mZaM
tt admin video label EJv04w-mZaM folklore      # tango_family, folklore, other, not_dance
tt admin video stamp EJv04w-mZaM live_music    # no_commercial_take, needs_human, none
tt admin video import uGwRPRusbC0 https://youtu.be/EJv04w-mZaM
tt admin clip edit sacada-boleo-combo --start 1:05 --end 1:20 --tags sacada,boleo
tt admin clip delete sacada-boleo-combo --yes
tt admin championships load --dry-run
```

An edit sends only the flags you give; `none` clears a field. Each is
validated before anything is written, so a bad date or an unknown singer is
a usage error even on `--dry-run`, and only the fields that actually change
are recorded. Edits, aliases, labels, stamps, features and channel states
undo with `tt admin undo`.

Some changes cannot be undone, and say so in their dry run before they run:
a dancer merge (the duplicate is deleted and its credits, couples, aliases,
titles, followers and pictures move, and the kept dancer's partnerships are
rebuilt from them; the record keeps a full snapshot of it), a clip delete, rejecting a channel as noise, a re-read, an import, and a
championships load. Merge, clip delete, deactivate and reject-noise ask for
`--yes`. `--replay` on a new alias re-reads up to 500 videos whose titles
carry the spelling, in the background.

### Pictures

```bash
tt admin image list dancer noelia-hurtado
tt admin image add dancer noelia-hurtado ./noelia.jpg --kind portrait --licence permission --credit "Ana Gómez" --dry-run
tt admin image add orchestra carlos-di-sarli https://example.org/di-sarli.jpg --kind cover --licence public_domain
tt admin image primary 57
tt admin image takedown 57 --reason "asked by the dancer" --yes
tt admin image review
tt admin image accept 61
tt admin image reject 62 --reason "not her"
```

A dancer, an orchestra or an event has a portrait and a cover. A couple's page
shows its dancers' portraits and a channel's shows its YouTube avatar, so
neither has a picture of its own, and a singer's page shows none yet; `tt`
says so rather than add a picture that would appear nowhere.

A path is uploaded; a URL is fetched by TangoTube, from the public web only:
http or https on the standard port, never an address inside a network, at
most three redirects, each checked again. Either way the picture is judged by
its bytes, not its name: JPEG, PNG or WebP, at most 10 MB, at least 200 pixels
on each side. `--licence` is required (`unknown`, `own_work`, `permission`,
`public_page`, `public_domain`, `cc`).

A new picture is shown when you say `--primary` or the page shows none of
that kind yet; the page's own attachment follows whichever picture is shown.
A picture the page shows from before pictures had records gets a record the
first time another replaces it, so it is never lost.

Adding cannot be undone: the picture stays on the record, and takedown takes
it off the page. `primary`, `takedown`, `accept` and `reject` undo with `tt
admin undo`. A takedown keeps the row, so the request stays answerable, and
the page falls back to the newest picture of that kind still up, or shows
none; the refusal without `--yes` and the dry run both say which.

### The review queues

```bash
tt admin review list --pile conflict          # or panel
tt admin review keep 162970 --dry-run         # keep the stored song, close the proposal
tt admin review accept 162970 --note "listened: it is Yira yira"
tt admin audio matches                        # matched and not yet judged
tt admin audio verdict 3120 wrong --song poema-canaro --dry-run
tt admin audio rerun 3120 --refingerprint
tt admin suggestions                          # pending; --status all for the rest
tt admin suggestion accept 16 --dry-run
tt admin reports                              # open; --status all for the rest
tt admin report resolve 3 --note "not tango: hidden"
tt admin report dismiss 4 --note "the credit is right"
tt admin tags pending
tt admin tag accept americana
tt admin tag block cross_system --yes
tt admin user show ana@example.com
tt admin user trust ana@example.com --trusted true --dry-run
tt admin user supporter ana@example.com --on
tt admin performance recredit k-shh3fWESA --dry-run
```

Each verb is the one the admin pages use, so the ledger, the notices and the
trust scores move exactly as they do there.

`review` settles the two coverage piles a person decides: Conflicts (a
confident panel disagrees with the stored song) and Panel leftover (a panel
named a song the matcher would not apply). The other piles are read with
`tt admin coverage --pile`. `keep` closes the proposal and stamps the song
manual, so no harvest moves it again; `reject` closes it; both undo.
`accept` writes the proposal and cannot be undone.

`audio verdict` records a fact check. `wrong` also takes audio's song off the
video, and `--song` names the right one (`--rendition` when you know the tune
and orchestra but not the take). A verdict is not undone with `tt admin undo`;
the song change it makes is a payload row, which the Payloads pile reverts.

`suggestion accept` applies a pending suggestion the way a trusted person's
applies: it goes on the video through the ledger, the author's trust score
rises a point, and they are told. A dancer credit is also added to the
performance's own credits, so it reaches the dancer's page at once; the watch
page and `tt suggest` do the same whenever a dancer credit applies.
`suggestion reject` lowers the author's score and tells them. Neither undoes.

`report resolve` and `dismiss` need `--note`: what was done, or why not. A
credit or fact hidden by reports shows again once its reports are closed;
`tt admin undo` reopens a report.

`tag accept` takes a tag into the vocabulary and onto every clip it was
suggested for. `block` takes it off every clip and asks for `--yes`; `unblock`
lets it be suggested again but does not put it back on the clips (the record
names them). None of the tag verbs undo.

`user` shows and sets a person's standing only: their trust tier and score,
whether they are always trusted, and whether they support TangoTube. It never
shows or changes an address, a role, a password or a token. Below a score of
5 a person's suggestions wait for review; from 5 they apply unverified; from
20, or with `--trusted true`, they apply verified. `trust` and `supporter`
undo.

`performance recredit` brings a performance's credits up to date with its
videos'. A dancer named on a video shows on the video at once, but reaches
their dancer page, pairings and search only through the performance's own
credits, which were built once and never again. Recredit adds the missing
ones and reindexes; it only adds, and names any credit no video carries any
more rather than removing it. It cannot be undone; report a wrong credit to
take it back. A suggestion's dancer credit recredits its performance on its
own; this is for credits that arrived another way.

### The payload ledger

```bash
tt admin payload status                                   # payloads landed, newest first
tt admin payload status --counts                          # and the rows waiting per queue (a few seconds)
tt admin payload export gender --limit 500 -o gender-2026-09-24.jsonl
#   ...decide each row: ref plus the keys its "decide" names...
tt admin payload import gender-2026-09-24-decided.jsonl --agent claude --dry-run
tt admin payload import gender-2026-09-24-decided.jsonl --agent claude
tt admin payload show 17 --state skipped
tt admin payload revert 17 --row dancer:9361 --yes        # one row
tt admin payload revert 17 --yes                          # the whole payload
```

The jumpstart loop over the API, for the six queues: gender, family, song,
credit, probe, withdrawal. `export` is the work list `jumpstart:export`
writes; each row carries `ref`, the context to decide from, and `decide`, the
shape of the answer. `-o` saves it one JSON object a line.

`import` lands a decided file (JSON lines, or one JSON array) through the same
importer as `jumpstart:import`. The queue comes from the file name, or
`--kind`. Every row is checked first: a line that is not JSON, a ref of the
wrong kind, a missing or unknown answer, or the same ref twice refuses the
whole file, each bad row named by its line. The rows are the payload's
identity, so importing them again applies nothing new. `--dry-run` runs the
import and rolls it back, and says what each row would do: apply (and what it
overwrites) or skip (and why: `human` when a person set it, `gone`,
`invalid`, `unchanged`, `kept`). Up to 5,000 rows an import.

`revert` puts back what a payload's applied rows overwrote, or one row's with
`--row`, as the Payloads pile on /admin/coverage does, and asks for `--yes`.
Each row goes back only where nothing has changed it since. A payload is its
own undo: `tt admin undo` does not take these verbs, and a reverted row stays
reverted even if the file is imported again.

probe, credit and withdrawal rows name appearances, whose ids only mean the
same thing in the database they came from: export and import against the same
one.

### Pipelines

```bash
tt admin jobs                                         # each failure: when, the job, its id, the error
tt admin job run --list                               # what tt runs, and the --arg each takes
tt admin job run reindex --arg mode=recent
tt admin job run reindex --arg mode=full --dry-run    # states the cost
tt admin job run channel-sync --arg channel=UCtdgMR0bmogczrZNpPaO66Q
tt admin job retry 3f2c6a2e-0d7b-4a53-9a55-5c1e8f0f4b1a
tt admin job discard 3f2c6a2e-0d7b-4a53-9a55-5c1e8f0f4b1a --yes
tt admin cron pause generate_sitemap --note "sitemap directory unwritable"
tt admin cron resume generate_sitemap                 # or tt admin undo the pause
tt admin rebuild --list                               # the steps and what each costs
tt admin rebuild partnerships --dry-run
tt admin rebuild partnerships --yes
```

`job run` starts one job from an allowlist, in the background: channel-sync
(one channel with `--arg channel=ID`, else every active, reviewed channel due),
place-new-videos (`full=true` for the nightly walk), reindex (`mode` recent,
full or listings), refresh-couples, harvest-descriptions, harvest-panels,
audio-sweep, fingerprint (`video=ID`, `force=true`), sitemap and
availability-check (`batch` 1 to 1000). Nothing else runs. Each answer says
what the job costs; every channel, the full placement walk, a full or
listings reindex and the audio sweep ask for `--yes`. The same job with the
same arguments is not queued while one waits or runs, an expensive one is not
started again within the hour, and one operator starts at most ten jobs in ten
minutes (exit 5). Recorded; not undoable.

`job retry` runs a failed job again, as the /good_job dashboard does. `job
discard` takes a job still waiting off the queue (a running one is left to
finish) and asks for `--yes`. `tt admin jobs` never shows a job's arguments,
and leaves out what an operator discarded.

`cron pause` switches a scheduled job off with GoodJob's own switch, so the
dashboard agrees; `resume` or `tt admin undo` switches it back on.

`rebuild` runs one step of `bin/rails rebuild:*` as a job: occasions,
editions, appearances, views, dances, links, pairings, establish,
partnerships, refresh-music, refresh-families, refresh-roles. Each builds only
what is not built and walks a whole table, so each states its cost and asks
for `--yes`. A dancer merge rebuilds the kept dancer's partnerships on its
own; `rebuild partnerships` is the whole catalogue's.

### Announcements

```bash
tt admin announcement list
tt admin announcement create --title "Mundial week" --body "Every final, as it happens." \
    --link /events/mundial-de-tango --link-text Watch --ends 2026-10-08 --dry-run
tt admin announcement end 12
```

The banner across the top of the site. One shows at a time, lowest
`--position` first, to `--audience` everyone, signed_in or signed_out, from
`--starts` (now) to `--ends` (until ended). `end` takes one down at once and
`tt admin undo` puts it back. TangoTube has no featured playlists on its pages
any more, so there is no verb for them; `tt admin video feature` features a
video.

## Open

```bash
tt open 2ByaUQXeAeo
tt open sacada-1-cuando-el-amor-muere
tt open noelia-hurtado
tt open 2ByaUQXeAeo --agent --jq .url
```

Opens the watch page, the clip, or a dancer, couple, orchestra, or song page in
your browser. Piped or under `--agent`, it
opens nothing and writes `{"url": …}`.

## Auth

```bash
tt auth login                  # browser: approve on tangotube.tv
tt auth login --device         # a code to type at tangotube.tv/device
tt auth login --admin          # operators: tt admin, for 90 days
tt auth login --with-token < token.txt
tt auth status
tt auth logout
```

A token is a personal access token, sent as `Authorization: Bearer tt_live_…`.
`tt auth login` makes one named "tt on <this machine>". You can also make one
in Settings → Tokens, and revoke any of them there.

tt looks for a token in this order:

1. `--token`
2. `TANGOTUBE_TOKEN`
3. the OS keyring (service `tangotube`)
4. `~/.config/tangotube/token`, mode 0600

`TANGOTUBE_NO_KEYRING=1` keeps it in the file, for headless boxes.
`tt auth status` says who you are, the token's scopes, and where it lives —
never the token. `tt auth logout` revokes the stored token on the site and
forgets it. A stored token the site turns down does not stop searching: tt
reads without it and tells you once on stderr.

With `--device --agent`, the code arrives as one JSON line on stderr
(`user_code`, `verification_uri`, `expires_in`) for the agent to hand to its
person while tt waits.

## Setup, skill, doctor

```bash
tt setup                # every agent tt can find
tt setup claude         # just this one
tt skill                # print SKILL.md
tt doctor               # binary, API, token, skill
tt commands --json      # the whole CLI, as data
tt mcp                  # an MCP server over stdio
tt version
tt upgrade              # checks GitHub for the latest release first
```

`tt upgrade` replaces a tt installed by the install script. A tt from mise or
`go install` is upgraded the same way it came; tt prints that command.

## Global flags

| Flag | Means |
|---|---|
| `--json` | Write the envelope, even at a terminal. |
| `--agent` | JSON, no colour, never prompt, never open a browser. |
| `--quiet` | Write only `data`. |
| `--jq EXPRESSION` | Filter `data` with jq, built in. Strings print bare. |
| `--limit NUMBER` | Results per page, up to 50. |
| `--api-url URL` | Another TangoTube, like `http://localhost:3000`. |
| `--token TOKEN` | This token, not the stored one. |
| `--verbose` | Each HTTP request on stderr. |

Environment: `TANGOTUBE_TOKEN`, `TANGOTUBE_API_URL`, `TANGOTUBE_DEV=1`
(`http://localhost:3000`), `TANGOTUBE_NO_KEYRING=1`, `NO_COLOR`,
`TT_NO_ANIMATION=1` (the couple on the home screen stay still),
`TT_IMAGES=kitty|iterm|none` (how the logo is drawn), `TT_NO_LINKS=1`
(ids and addresses are not clickable).

## Links

At a terminal, ids are links. A video id opens its TangoTube watch page; a
clip id opens the clip; the VIDEO column of a clip opens the watch page at
the loop. `tt video show` gives both addresses, TangoTube and YouTube. They
are OSC 8 hyperlinks, which iTerm2, Ghostty, WezTerm, kitty, GNOME Terminal,
Windows Terminal and VS Code open with a click (⌘-click or Ctrl-click in
some); other terminals show the text. Piped output has no links, only the
`watch_url` and `clip_url` fields in the JSON.

## The logo

`tt` with nothing after it draws the TangoTube logo: the couple in their disc
and the wordmark under them. Where the terminal can show images, it is a real
image, drawn at the screen's own resolution from the logo's geometry and the
wordmark's outlines: kitty's graphics protocol in kitty, Ghostty, WezTerm and
Konsole; iTerm2's inline images in iTerm2 (and WezTerm). Elsewhere, and inside
tmux, it is drawn in half-block characters on a dark panel of its own.
`TT_IMAGES=kitty`, `iterm` or `none` overrides the guess.

Where a person is watching, the couple turn one giro first, a little over a
second. There is no animation when stdin or stdout is not a terminal, under
`--agent` or `--json`, with `NO_COLOR`, in `CI`, or with `TT_NO_ANIMATION=1`.
Smaller terminals get the couple alone, with the name beside them in words.

`tt dance` is the couple dancing one tango, about three minutes. Ctrl-C says
gracias.

## The envelope

```json
{
  "ok": true,
  "data": {"videos": [], "total": 0, "next_cursor": null, "filters": {}},
  "summary": "12 videos · Carlos Di Sarli · Noelia Hurtado · 1941–1954",
  "breadcrumbs": ["tt video show 2ByaUQXeAeo"]
}
```

```json
{
  "ok": false,
  "error": {
    "code": "not_found",
    "message": "No video with that id",
    "hint": "tt search \"di sarli\"",
    "retryable": false
  }
}
```

In JSON modes errors go to stdout, so a program reads one stream. At a
terminal they go to stderr.

## Exit codes

| Exit | Code | When |
|---|---|---|
| 0 | | It worked. |
| 1 | `usage` | A flag, argument, or time was wrong. |
| 2 | `not_found` | No such video, clip, dancer, or name. |
| 3 | `auth` | No token, or a revoked one. |
| 4 | `forbidden` | Not yours to change. |
| 5 | `rate_limit` | Too many requests. Wait and retry. |
| 6 | `network` | TangoTube could not be reached. |
| 7 | `api` | TangoTube answered with something unexpected. |
