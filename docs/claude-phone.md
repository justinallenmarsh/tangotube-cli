# Use TangoTube from Claude on your phone

TangoTube runs an MCP server at `https://tangotube.tv/mcp`, so Claude on your
phone, the Claude desktop app and claude.ai can search videos, open a
performance, look up a dancer, keep your likes, playlists and practice list,
and help name videos. It serves the same tools as `tt mcp`, with the same
names and arguments.

## Add it

1. In Claude, open **Settings → Connectors**.
2. Choose **Add custom connector**.
3. Name it TangoTube, and give the URL `https://tangotube.tv/mcp`.
4. Save, then choose **Connect**. Connectors you add on claude.ai also appear
   on your phone.

## Sign in

Connecting opens tangotube.tv to sign you in. The **Connect Claude** page
shows who you are signed in as, what Claude may do (read, and save, change
and suggest as you), and that it sends you back to claude.ai. Choose
**Allow**.

The token Claude gets appears in **Settings → Tokens** under the name
"Claude". Revoke it there and Claude asks you to connect again. It works only
at `tangotube.tv/mcp`, nowhere else.

## Search only, without an account

Give the URL `https://tangotube.tv/mcp/public` instead. Claude can search and
read the catalogue, and never asks you to sign in; your likes, playlists and
clips need `https://tangotube.tv/mcp`.

## For operators

An operator connecting Claude sees an **Operator tools** switch on the
Connect page. With it on, Claude also gets the `tt admin` tools, and the token
lapses after 90 days. Any operator tool that changes the catalogue only shows
what it would do until Claude calls it again with `confirm: true`; Claude
should ask you first. Every change is recorded and, where it can be, undone
with `admin_undo`.

## What it cannot do

- Import a YouTube Takeout export: that reads files on your computer. Use
  `tt import youtube` there.
- Add a picture from your phone's files: give Claude an https address instead,
  or use `tt admin image add` with a file.
