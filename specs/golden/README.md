# Golden cases

Each directory here is one golden case. It is part of the specification: the
expected files define opencligen's output byte for byte, and the tests fail
on any difference.

| File | Content |
|---|---|
| `spec.json` | The input OpenCLI description. |
| `urfave.go.golden` | `opencligen urfave --package <package> --source specs/golden/<case>/spec.json spec.json` |
| `markdown.md.golden` | `opencligen markdown spec.json` |
| `man.1.golden` | `opencligen man spec.json` |

The directory name is the case name. Use plain, descriptive words joined by
hyphens, such as `all-features`; avoid idioms and wordplay, which readers
whose first language is not English may not recognise. The Go package name
of the urfave output is the case name without hyphens (`allfeatures`), so it
must then be a valid Go identifier.

## Cases

| Case | Covers |
|---|---|
| [all-features](all-features/) | A fictional cooking CLI named `kitchen` (commands `cook` and `pantry`) that uses every feature: info contact and license, conventions, recursive and built-in options, aliases, categories, `x-usage`, `x-documentation`, examples, exit codes, interactive and hidden commands, hidden options, typed flags (int with range and default, uint, repeatable float, duration, string with accepted values, boolean with a value, presence flag), required flags, typed and variadic positional arguments, `x-go-name`, and a group command that takes arguments. |

The all-features case is also built and run by `gen/urfave/e2e_test.go`,
which checks the runtime behaviour of the generated CLI.

## Changing or adding cases

- To add a case, create `<case>/spec.json` and run `make update-golden`; the
  tests pick up every directory automatically.
- After an intended change to a generator, run `make update-golden` and
  review the diff of the `.golden` files: it is the user-visible change.
- Never edit `.golden` files by hand.
