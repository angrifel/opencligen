# Golden cases

Each directory here is one golden case: an input description and what
opencligen must produce from it. The cases are part of the specification.

| File | Content | Compared |
|---|---|---|
| `spec.json` | The input OpenCLI description. | |
| `structure.json` | The command tree that the generated urfave/cli code builds (see below). | as data |
| `markdown.md.golden` | `opencligen markdown spec.json` | byte for byte |
| `man.1.golden` | `opencligen man spec.json` | byte for byte |

## Generated Go code: structure, not text

The Go output is not compared as text, so reordering or reformatting the
generated source never fails a test. Instead, `gen/urfave/structure_test.go`
compiles the generated code, walks the urfave/cli command tree that `New`
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

## Cases

| Case | Covers |
|---|---|
| [all-features](all-features/) | A fictional cooking CLI named `kitchen` (commands `cook` and `pantry`) that uses every feature: info contact and license, conventions, recursive and built-in options, aliases, categories, `x-usage`, `x-documentation`, examples, exit codes, interactive and hidden commands, hidden options, typed flags (int with range and default, uint, repeatable float, duration, string with accepted values, boolean with a value, presence flag), required flags, typed and variadic positional arguments, `x-go-name`, and a group command that takes arguments. |
| [nested-commands](nested-commands/) | A fictional repository tool named `repo` with commands four levels deep (`repo remote branch list`): recursive options declared at the root and at intermediate levels, inherited only by their own subtree; a recursive option redeclared further down; a root option that is not inherited; the same command name (`list`) under three different parents, giving three handlers; aliases and options at several levels; hidden commands at the third level and a hidden group; a group command that takes arguments and has a subcommand; `x-go-name` and `x-category` below the root. |
| [minimal](minimal/) | A root command with nothing else: it is runnable and its handler is `Root`. |

The all-features case is also built with a hand-written `Handlers`
implementation and run by `gen/urfave/e2e_test.go`, which checks runtime
behaviour such as defaults, range checks and argument assignment.

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
  review the diff of `structure.json` and the `.golden` files: it is the
  user-visible change.
- Never edit `structure.json` or `.golden` files by hand.
