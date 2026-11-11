# Golden cases

Each directory here is one golden case: an input description and what
opencligen must produce from it. The cases are part of the specification.

| File | Content |
|---|---|
| `spec.json` | The input OpenCLI description. |
| `structure.json` | The command tree that the generated urfave/cli code builds. |
| `markdown.json` | The outline of `opencligen markdown spec.json`: sections, paragraphs, code blocks, tables and lists. |
| `man.json` | The outline of `opencligen man spec.json`: the `.TH` header, sections, subsections, paragraphs, literal blocks and tagged entries. |
| `runs.txt` | Optional. Command lines for the case's reference CLI and what they must produce (see "Reference CLIs" below). |

No output is compared as text. Each test reads the output back into
structure, checks it against `spec.json`, and compares the structure with the
expected `.json` file as data. Layout changes (reordered Go functions,
spacing, table padding, which roff requests are used) never fail a test;
changes of content or structure do, and show up in review as a diff of the
`.json` files.

## Where the tests are

Every case directory is picked up automatically by `golden.Cases`
(`internal/golden/golden.go`), and `golden.Assert` compares a result with the
case's expected file (or rewrites it with `-update`).

| Expected file | Test |
|---|---|
| `structure.json` | `TestGoldenCases` in `internal/gen/urfave/golden_test.go` |
| `markdown.json` | `TestGoldenCases` in `internal/gen/markdown/golden_test.go` |
| `man.json` | `TestGoldenCases` in `internal/gen/man/golden_test.go` |
| (none; renders each page) | `TestGroff` in `internal/gen/man/man_test.go` |
| `runs.txt` | `TestReferenceRuns` in `internal/gen/urfave/runs_test.go` |

Run only the golden cases with

```sh
go test ./internal/gen/... -run 'TestGoldenCases|TestGroff|TestReferenceRuns' -v
```

Each case is a subtest, for example `TestGoldenCases/nested-commands`.

## Generated Go code

`internal/gen/urfave/golden_test.go` compiles the generated code, walks the urfave/cli command tree that `New`
returns, and runs command lines against it. It checks three things:

1. **Properties derived from `spec.json`**, for every command at every depth:
   name, aliases, usage, category, hidden, positional argument usage, version
   and short option grouping on the root, examples, exit codes and
   documentation links in the description; and for every option, the flag
   type (from `x-type` and the arity), names, usage, category, required,
   hidden, inherited by subcommands (recursive) or not, default, and whether
   values are validated.
2. **Behaviour on the command line**, for every runnable command:
   - it runs with valid arguments and reaches the expected `Handlers` method
     (`x-go-name`, or the command path in CamelCase; unique across the tree);
   - every recursive option of every ancestor is accepted;
   - a non-recursive option of each ancestor is rejected.
3. **`structure.json`**: the same tree written out as data, so a change of
   structure shows up in review. Each command records its name, aliases,
   usage texts, category, version, argument usage, hidden, whether it is
   runnable and its handler; each flag records its type, names, usage,
   category, required, hidden, inherited, default and validated.

Checks 1 and 2 also run on the upstream examples in `testdata/upstream`,
whose git description has 251 commands.

## Markdown

`internal/gen/markdown/golden_test.go` reads the document back with a small
Markdown reader (`outline_test.go`) and checks, against `spec.json`:

- the title, the summary, and a command index that lists every visible
  command in order, at its depth, linking to its section;
- exactly one section per visible command and none for hidden ones, each with
  the summary, aliases, category and the usage block (`x-usage`, or the
  generated synopsis);
- the Options, Inherited options, Arguments, Subcommands and Exit codes
  tables: one row per visible item, in order, with names, value placeholder,
  type, accepted values, range, default, "Required.", group and description;
- examples;
- that every internal link resolves to a heading.

## Man page

`internal/gen/man/golden_test.go` reads the page back with a roff reader
(`roff_test.go`) and checks the `.TH` header, which sections appear and in
what order, NAME, SYNOPSIS, DESCRIPTION, the option and argument entries of
the root, one COMMANDS subsection per visible command in order (synopsis,
description, aliases, option, argument, example and exit status entries),
and EXIT STATUS. The reader knows only the requests and escapes the generator
uses; anything else fails the test, so supporting a new one is a deliberate
change. `TestGroff` additionally renders every page with `groff -ww` when it
is installed and fails on warnings.

Both document checks also run on the upstream git and curl examples.

## Reference CLIs

For each case with a `runs.txt`, `TestReferenceRuns` builds a reference CLI:
the generated urfave/cli code plus a generated `Handlers` implementation in
which every handler prints

```
handler: <method name>
args: [<positional values>]
--<flag>: <value>        one line per flag that applies to the command
```

and replays the command lines of `runs.txt`:

```
# Lines starting with "#" are comments.
$ cook --batch 1 pasta              the arguments, after the program name
handler: Cook                       a line that must appear in the output
--heat: 5

$ cook --batch 1 --heat 10 pasta
error: invalid value 10 for --heat  the run must fail with this in its error output

$ --version
output: kitchen version 1.2.3       text that must appear somewhere in the output
```

A run without `error:` lines must succeed. Output lines only need to appear,
so a run lists just the flags it is about; `error:` and `output:` match
substrings, so they keep working when urfave/cli rewords its own messages.
Arguments are split on spaces; quoting is not supported.

## Cases

| Case | Covers |
|---|---|
| [all-features](all-features/) | A fictional cooking CLI named `kitchen` (commands `cook` and `pantry`) that uses every feature: info contact and license, conventions, recursive and built-in options, aliases, categories, `x-usage`, `x-documentation`, examples, exit codes, interactive and hidden commands, hidden options, typed flags (int with range and default, uint, repeatable float, duration, string with accepted values, boolean with a value, presence flag), required flags, typed and variadic positional arguments, `x-go-name`, and a group command that takes arguments. |
| [nested-commands](nested-commands/) | A fictional repository tool named `repo` with commands four levels deep (`repo remote branch list`): recursive options declared at the root and at intermediate levels, inherited only by their own subtree; a recursive option redeclared further down; a root option that is not inherited; the same command name (`list`) under three different parents, giving three handlers; aliases and options at several levels; hidden commands at the third level and a hidden group; a group command that takes arguments and has a subcommand; `x-go-name` and `x-category` below the root. |
| [minimal](minimal/) | A root command with nothing else: it is runnable and its handler is `Root`. |

Every case has a `runs.txt`: defaults, aliases, value and argument checks
(all-features), inheritance and rejection of options across four levels
(nested-commands), and a root that runs itself (minimal).

## Naming cases

The directory name is the case name. Use plain, descriptive words joined by
hyphens, such as `nested-commands`; avoid idioms and wordplay, which readers
whose first language is not English may not recognise. The Go package name
of the generated code is the case name without hyphens (`nestedcommands`), so
that must be a valid Go identifier.

## Changing or adding cases

- To add a case, create `<case>/spec.json` and run `make update-golden`; the
  tests pick up every directory automatically.
- After an intended change to a generator, run `make update-golden` and
  review the diff of the `.json` files: it is the user-visible change.
- Never edit the expected `.json` files by hand.
