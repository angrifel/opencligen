# opencligen specifications

Everything that defines what opencligen does, as opposed to how it is
implemented, lives in this directory. When the code and these files disagree,
these files win: fix the code, or change the spec deliberately.

| Path | What it specifies |
|---|---|
| [opencligen.json](opencligen.json) | opencligen's own command line (OpenCLI 0.1): commands, flags, defaults, exit codes. The CLI code is generated from it. |
| [cli.md](cli.md), [opencligen.1](opencligen.1) | Reference documentation of the command line, generated from `opencligen.json`. Do not edit; run `make generate`. |
| [extensions.md](extensions.md) | The `x-` metadata opencligen understands (types, defaults, ranges, command metadata), its strictness rules and urfave/cli limitations. |
| [golden/](golden/) | Golden cases: input descriptions with the expected structure of every output (generated Go command tree, Markdown outline, man page outline). See [golden/README.md](golden/README.md). |

Two related inputs live elsewhere, because they are not opencligen's own
specification:

- `../schema/<version>/schema.json`: the upstream OpenCLI JSON schemas,
  unmodified. They stay next to the Go package that embeds them.
- `../testdata/upstream/`: the upstream OpenCLI examples, a third-party test
  corpus.
