# Style

- `gofmt`. `make check` fails on anything it would change.
- No unused exports. If nothing outside the package calls it, lower-case it.
- Help strings are sentences a dancer or an agent can use: "Only videos with
  this dancer: a NAME or slug." — not "dancer filter (string)". End them with
  a period.
- Say performance, orchestra, milonga, sacada, couple: the app's own nouns.
  Never content, creators, platform, leverage, seamless, or AI-powered.
- Examples are real invocations with real ids and slugs, never `foo`.
- Colour only at a terminal, only from the palette in
  `internal/output/style.go`, and never under `NO_COLOR`, `--json`,
  `--agent`, or a pipe.
- Every error says what happened and, in `hint`, the command that fixes it.
- A flag exists only if the API honours it. No flag that is quietly ignored.
- Nothing here imports the TangoTube app. This directory must stand alone as
  its own repository.
