# Upstream OpenCLI examples

Unmodified copies of the example descriptions from
https://github.com/spectreconsole/open-cli/tree/e42891559d59e84b1da8a8901450fa37784597ed/examples,
used as a test corpus. Refresh them with `make update-upstream`.

Known issues in this corpus, which the tests assert on:

- `dotnet.json` declares the same option name twice in a few commands, for
  example `dotnet test` gives `-t` to both `--list-tests` and `--target`.
- `ffmpeg.json` uses `x-type`, `x-range` and `x-default` with its own
  conventions: C limit constants (`-allow-c-limits`) and types or default
  values that cannot be typed (`-lenient`).
