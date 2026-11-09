# Golden cases

Each directory here is one golden case. It is part of the specification: the
expected files define opencligen's output byte for byte, and the tests fail
on any difference.

| File | Content |
|---|---|
| `spec.json` | The input OpenCLI description. |
| `urfave.go.golden` | `opencligen urfave --package <case> --source specs/golden/<case>/spec.json spec.json` |
| `markdown.md.golden` | `opencligen markdown spec.json` |
| `man.1.golden` | `opencligen man spec.json` |

The directory name is the case name and the Go package name of the urfave
output, so it must be a valid Go identifier.

## Cases

| Case | Covers |
|---|---|
| [kitchen](kitchen/) | Every feature: info contact and license, conventions, recursive and built-in options, aliases, categories, `x-usage`, `x-documentation`, examples, exit codes, interactive and hidden commands, hidden options, typed flags (int with range and default, uint, repeatable float, duration, string with accepted values, boolean with a value, presence flag), required flags, typed and variadic positional arguments, `x-go-name`, and a group command that takes arguments. |

The kitchen case is also built and run by `gen/urfave/e2e_test.go`, which
checks the runtime behaviour of the generated CLI.

## Changing or adding cases

- To add a case, create `<case>/spec.json` and run `make update-golden`; the
  tests pick up every directory automatically.
- After an intended change to a generator, run `make update-golden` and
  review the diff of the `.golden` files: it is the user-visible change.
- Never edit `.golden` files by hand.
