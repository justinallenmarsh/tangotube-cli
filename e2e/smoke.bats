#!/usr/bin/env bats
# Smoke tests: the built binary against the recorded fixture API.
# make smoke builds ./bin/tt and runs this; no database, no network.

setup_file() {
  export PORT=4599
  export TANGOTUBE_API_URL="http://127.0.0.1:$PORT"
  export TANGOTUBE_NO_KEYRING=1
  export HOME="$BATS_FILE_TMPDIR/home"
  mkdir -p "$HOME"
  go build -o "$BATS_FILE_TMPDIR/fixtureserver" ./e2e/fixtureserver
  "$BATS_FILE_TMPDIR/fixtureserver" -addr "127.0.0.1:$PORT" 2>/dev/null &
  echo $! > "$BATS_FILE_TMPDIR/server.pid"
  for _ in $(seq 50); do curl -fs "$TANGOTUBE_API_URL/api/v1/tags" >/dev/null && break; sleep 0.1; done
}

teardown_file() {
  kill "$(cat "$BATS_FILE_TMPDIR/server.pid")" 2>/dev/null || true
}

tt() { ./bin/tt "$@"; }

@test "tt with no args names three next commands" {
  run tt --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'"breadcrumbs":["tt search'* ]]
}

@test "search writes the envelope with breadcrumbs" {
  run tt search "di sarli" --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'"ok":true'* ]]
  [[ "$output" == *'"breadcrumbs"'* ]]
}

@test "search by technique returns clips" {
  run tt search "sacada" --technique sacada --json --jq '.videos[0].clips[0].tags[0]'
  [ "$status" -eq 0 ]
  [ "$output" = "sacada" ]
}

@test "video show on an id from search" {
  id="$(tt search "noelia" --jq '.videos[0].id')"
  run tt video show "$id" --json
  [ "$status" -eq 0 ]
}

@test "a missing video exits 2" {
  run tt video show nope --json
  [ "$status" -eq 2 ]
  [[ "$output" == *'"code":"not_found"'* ]]
}

@test "a write without a token exits 3" {
  run tt clip create 2ByaUQXeAeo --start 1:12 --end 1:18 --tag sacada --agent
  [ "$status" -eq 3 ]
}

@test "commands and doctor answer in JSON" {
  run tt commands --json
  [ "$status" -eq 0 ]
  run tt doctor --json
  [ "$status" -eq 0 ]
}

@test "skill prints SKILL.md" {
  run tt skill
  [ "${lines[1]}" = "name: tangotube" ]
}

@test "the catalogue lists page by slug" {
  run tt dancers --champion --jq '.dancers[0].slug'
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  run tt champions --year 2005 --jq '.champions[0].year'
  [ "$output" = "2005" ]
}

@test "facets, resolve and home answer" {
  run tt facets --orchestra "di sarli" --jq '.facets.recorded | length > 0'
  [ "$output" = "true" ]
  run tt resolve "di sarli noelia" --jq '.reading[0].value'
  [ "$output" = "carlos-di-sarli" ]
  run tt home --jq '.sections | length > 0'
  [ "$output" = "true" ]
}

@test "search browses with a sort alone" {
  run tt search --sort hidden-gems --kind class --json
  [ "$status" -eq 0 ]
  run tt search --sort loudest --json
  [ "$status" -eq 1 ]
}

@test "event, channel, versions and the performance session" {
  run tt event show planetango --json
  [ "$status" -eq 0 ]
  run tt channel show UCCoOxQMnmwZ-jhezbLfSUgQ --json
  [ "$status" -eq 0 ]
  run tt song versions todo-es-amor-fulvio-salamanca --jq '.versions | length'
  [ "$output" = "2" ]
  run tt video show bUFFLZVvttk --jq '.performance_session.total'
  [ "$output" = "3" ]
  run tt performance show cPJ3MjWDUVY --jq '.dance.videos | length'
  [ "$output" = "3" ]
  run tt partners sebastian-achaval --depth 2 --jq '[.nodes[].ring] | max'
  [ "$output" = "2" ]
}

@test "helping: the queue, a suggestion that says where it went, a report" {
  run tt queue --proposed --jq '.videos[0].open[0].state'
  [ "$output" = "proposed" ]
  run tt suggest KICHdm0zuCQ --dancer roxana-suarez --role follower --token tt_live_fixture --jq '.outcome'
  [ "$output" = "queued" ]
  run tt suggest KICHdm0zuCQ --dancer roxana-suarez --json
  [ "$status" -eq 3 ]
  run tt report SkTQuLhQG9A --kind not_tango --jq '.report.about'
  [ "$output" = "video" ]
}

@test "operating: admin needs an admin token, previews, queries, and undoes" {
  run tt admin jobs --token tt_live_fixture --json
  [ "$status" -eq 4 ]
  run tt admin video hide EJv04w-mZaM --dry-run --token tt_live_fixture_admin --jq '.dry_run'
  [ "$output" = "true" ]
  run tt admin query "SELECT count(*) FROM videos" --token tt_live_fixture_admin --jq '.columns | length'
  [ "$output" = "2" ]
  run tt admin query "SELECT email FROM users" --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin undo 1 --token tt_live_fixture_admin --jq '.undone'
  [ "$output" = "true" ]
}

@test "operating records: edits preview, destructive verbs want --yes" {
  run tt admin dancer edit noelia-hurtado --bio "Born in Buenos Aires." --dry-run --token tt_live_fixture_admin --jq '.would.bio[1]'
  [ "$output" = "Born in Buenos Aires." ]
  run tt admin dancer merge bailaron-neolia-hurtado into noelia-hurtado --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin channel deactivate UCCoOxQMnmwZ-jhezbLfSUgQ --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin championships load --dry-run --token tt_live_fixture_admin --jq '.undoable'
  [ "$output" = "false" ]
}

@test "operating images: add previews from a URL, a private address is refused, takedown wants --yes" {
  run tt admin image add dancer noelia-hurtado https://example.org/noelia.jpg --licence permission --dry-run --token tt_live_fixture_admin --jq '.undoable'
  [ "$output" = "false" ]
  run tt admin image add dancer noelia-hurtado http://169.254.169.254/latest/ --licence permission --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin image takedown 2 --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin image list dancer noelia-hurtado --token tt_live_fixture_admin --jq '.showing.portrait'
  [ "$output" = "2" ]
}

@test "moderating: review piles, a suggestion previewed, a block wants --yes, a person's standing" {
  run tt admin review list --token tt_live_fixture_admin --jq '.bills[0].commands[0]'
  [ "$output" = "tt admin review keep 162970" ]
  run tt admin suggestion accept 16 --dry-run --token tt_live_fixture_admin --jq '.would.recredited[1]'
  [ "$output" = "daiana-guspero" ]
  run tt admin tag block cross_system --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin report resolve 3 --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin user show dancer@tangotube.test --token tt_live_fixture_admin --jq '.user | has("email")'
  [ "$output" = "false" ]
  run tt admin performance recredit k-shh3fWESA --dry-run --token tt_live_fixture_admin --jq '.missing[0].dancer_slug'
  [ "$output" = "daiana-guspero" ]
}

@test "the ledger: export to a file, a dry-run import, a bad row refused, revert wants --yes" {
  d="$BATS_TEST_TMPDIR"
  run tt admin payload export gender --limit 2 -o "$d/gender-2026-09-24.jsonl" --token tt_live_fixture_admin --jq '.rows | length'
  [ "$output" = "2" ]
  [ "$(wc -l < "$d/gender-2026-09-24.jsonl")" -eq 2 ]
  printf '{"ref":"dancer:11467","gender":"male"}\n{"ref":"dancer:7725","gender":"male"}\n' > "$d/gender-2026-09-24-decided.jsonl"
  run tt admin payload import "$d/gender-2026-09-24-decided.jsonl" --agent claude --dry-run --token tt_live_fixture_admin --jq '.tally.applied'
  [ "$output" = "2" ]
  run tt admin payload import "$d/gender-2026-09-24-decided.jsonl" --agent claude --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  printf '{"ref":"dancer:1"\n' > "$d/gender-broken.jsonl"
  run tt admin payload import "$d/gender-broken.jsonl" --agent claude --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  [[ "$output" == *"line 1"* ]]
  run tt admin payload revert 17 --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin payload status --token tt_live_fixture_admin --jq '.counted'
  [ "$output" = "false" ]
  run tt admin payload status --counts --token tt_live_fixture_admin --jq '.queues.gender > 0'
  [ "$output" = "true" ]
  run tt admin actions --kinds --token tt_live_fixture_admin --jq '[.kinds[] | select(.kind == "payload.revert")][0].undoable'
  [ "$output" = "false" ]
}

@test "pipelines: a failure to retry, an expensive run wants --yes, a pause previewed, a rebuild's cost, an announcement" {
  run tt admin jobs --token tt_live_fixture_admin --jq '.failures[0] | has("args") or has("arguments")'
  [ "$output" = "false" ]
  run tt admin jobs --token tt_live_fixture_admin --jq '[.failures[].error_class] | index("GenerateSitemapJob::UnwritableSitemapDirectory") != null'
  [ "$output" = "true" ]
  run tt admin job run --list --token tt_live_fixture_admin --jq '[.runs[].name] | index("reindex") != null'
  [ "$output" = "true" ]
  run tt admin job run reindex --arg mode=full --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin job run reindex --arg mode=full --dry-run --token tt_live_fixture_admin --jq '.expensive'
  [ "$output" = "true" ]
  run tt admin job run sitemap --token tt_live_fixture_admin --jq '.action.action'
  [ "$output" = "job.run" ]
  run tt admin job run reindex --arg mode --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  [[ "$output" == *"NAME=VALUE"* ]]
  run tt admin cron pause generate_sitemap --dry-run --token tt_live_fixture_admin --jq '.would.enabled[1]'
  [ "$output" = "false" ]
  run tt admin rebuild partnerships --token tt_live_fixture_admin --json
  [ "$status" -eq 1 ]
  run tt admin rebuild partnerships --dry-run --token tt_live_fixture_admin --jq '.job'
  [ "$output" = "RebuildStepJob" ]
  run tt admin announcement create --title "Mundial week" --body "Every final, as it happens." --dry-run --token tt_live_fixture_admin --jq '.announcement.title'
  [ "$output" = "Mundial week" ]
  run tt admin announcement list --token tt_live_fixture_admin --jq '.announcements[0].state'
  [ "$output" = "ended" ]
}
