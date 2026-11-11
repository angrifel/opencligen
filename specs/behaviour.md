# Behaviour

Rules that opencligen follows and that are not visible from the OpenCLI
specification or [extensions.md](extensions.md). The golden cases
([golden/](golden/)) show them on concrete input; this page states them in
general.

## Reading a description

### Arity and values

- An argument without `arity` takes exactly one value (minimum 1,
  maximum 1, the schema defaults). A bound that is left out keeps its
  default: `{"minimum": 0}` means zero or one value, and `{"minimum": 2}`
  alone is an error, because its maximum is still 1.
- An argument that is not `required` may be left out entirely: its effective
  minimum is 0, whatever its arity says. A required argument's minimum is its
  arity minimum.
- An option "takes several values" when it has more than one argument, or
  when its first argument's arity maximum is above 1. Only the first
  argument's `acceptedValues` apply to the option.
- An arity minimum above its maximum is an error.

### Option names

- Names are compared without their prefix: `--verbose`, `-verbose` and
  `/verbose` are all `verbose`. An option's name and aliases that collapse to
  the same name count once.
- Two different options of one command may not share a name after this
  normalisation. The error points at the second option.

### Commands

- A command's summary is the first line of its description. For the root,
  `info.summary` wins when it is set, and the description falls back to
  `info.description`.
- A command is *runnable* when it has no subcommands, or when it takes
  positional arguments. Other commands only dispatch to their subcommands.
- Subcommand names and aliases must be unique among siblings.

### Inherited options

- A command inherits the `recursive` options of all its ancestors, nearest
  ancestor first.
- An option that is declared again, with the same set of names, by the
  command itself (recursive or not) or as a recursive option of a nearer
  ancestor replaces the farther one: it is inherited once, from the nearest
  declaration. A non-recursive option of a nearer ancestor does not replace
  anything, since it does not reach the command; the farther recursive
  option is still inherited.
- A command may reuse just some of the names of an inherited option (git's
  `grep -h` next to the inherited `--help`/`-h`). The description is valid;
  whether an output can express it is up to that output (see "urfave/cli
  output").

## Diagnostics

- Each diagnostic is printed as `file:line:col: severity: message`, with
  severity `error` or `warning`. Diagnostics are sorted by position.
- The position is the start of the JSON value the problem is about: for an
  x- metadata problem, the `value` of that metadata entry; for a duplicate
  name, the option's `name` (or `aliases`). When the value does not exist,
  for example a missing required property, the position is that of the
  nearest enclosing value that does.
- For invalid JSON, the position is the offending character.
- Schema violations are reported one per failing rule, prefixed "does not
  conform to the OpenCLI schema:".
- With `--lenient`, problems that have a fallback are warnings; the rest stay
  errors. Warnings never change the exit code. Each option or argument gets
  at most one x- metadata diagnostic: the first problem found, checked in
  the order duplicates, `x-type`, `x-range`, `x-default`.

### Where each diagnostic points

| Problem | JSON pointer |
|---|---|
| Invalid JSON | the offending character |
| Unsupported OpenCLI version | `/opencli` |
| Schema violation | the value that violates the rule, or its nearest existing parent |
| Empty command name | the command's `name` |
| Duplicate subcommand name or alias | the second subcommand's `name`, also when the clash is an alias |
| Two options sharing a name | the second option's `name`, or its `aliases` array when the shared name is an alias |
| Option with an empty name or alias | the option |
| Arity minimum above maximum | the argument's `arity` |
| Problem in `x-type`, `x-default` or `x-range` | that metadata entry's `value`; for an array default, the whole array |
| Problem in `x-category`, `x-usage` or `x-documentation` | that metadata entry's `value` |
| Option separator urfave/cli cannot use | `/conventions/optionSeparator` |
| Option with several arguments per use | the option's `arguments` |
| Shadowed inherited option | the subcommand option's `name` |
| Invalid `x-go-name` | the command's `metadata` |
| Two commands with the same handler name | the second command |

### Messages

Messages about one option or argument start with the command path and the
item, `<command path>: option "<name>": ` or `<command path>: argument
"<name>": `, followed by one of the x- metadata messages below. Lenient
warnings end with `; treating it as untyped`.

| Area | Message |
|---|---|
| Document | `invalid JSON: <reason>` |
| | `unsupported OpenCLI version "<v>" (supported: <versions>)` |
| | `does not conform to the OpenCLI schema: <rule>` |
| Commands | `command name is empty` |
| | `<path>: subcommand name "<n>" is declared more than once` |
| | `<path>: options "<a>" and "<b>" both declare the name "<n>"` |
| | `<path>: option "<o>" has an empty name or alias` |
| | `<path>: argument "<a>" has arity minimum <lo> greater than maximum <hi>` |
| | `<path>: option "<o>" argument "<a>" has arity minimum <lo> greater than maximum <hi>` |
| | `<path>: x-category must be a string` (likewise `x-documentation`) |
| | `<path>: x-usage must be a string or an array of strings` |
| x- metadata | `<name> is declared more than once` |
| | `x-type must be a string` |
| | `unsupported x-type "<t>" (supported: string, int, uint, float, boolean, duration)` |
| | `x-type "<t>" needs the option to take an argument` |
| | `x-type "<t>" cannot be used for several values` |
| | `acceptedValues can only be used with x-type string, not "<t>"` |
| | `x-range must be an object with a minimum and/or maximum: <reason>` |
| | `x-range needs a minimum or a maximum` |
| | `x-range can only be used with x-type int, uint or float, not "<t>"` |
| | `x-range minimum: <value message>` (likewise `maximum`) |
| | `x-range minimum <lo> is greater than maximum <hi>` |
| | `x-default must be an array for several values` |
| | `x-default: <value message>` |
| Value messages | `"<text>" is not a valid <type>` |
| | `<json> is not a number` |
| | `"<text>" is a C limit constant; enable AllowCLimits (--allow-c-limits) to accept it` |
| | `<json> is not a valid string` / `is not a valid boolean` |
| | `<json> is not a duration string such as "1m30s"` |
| | `"<text>" is not one of the accepted values (<a>, <b>)` |
| | `<value> is below the x-range minimum <lo>` / `is above the x-range maximum <hi>` |
| urfave/cli output | `option separator "<s>" is not supported by urfave/cli, which only accepts " " and "="` |
| | `option "<o>" takes <n> arguments per use, which urfave/cli flags cannot express` |
| | `<path>: option "<o>" reuses the name "<n>" of the recursive option "<p>"; urfave/cli would drop "<p>" entirely in this subcommand` |
| | `<path>: x-go-name must be an exported Go identifier, got <json>` |
| | `commands "<a>" and "<b>" both map to handler method <M>; set x-go-name on one of them` |
| | `invalid package name "<p>"` |
| Generated code, at run time | `<command>: expected at least <n> argument(s), got <m>` (likewise `at most`) |
| | `<command>: invalid value "<v>" for <arg> (accepted: <a>, <b>)` |
| | `invalid value <v> for <name> (minimum: <lo>)` (likewise `maximum`) |
| | `invalid value "<v>" for <name> (expected true or false)` |
| | `invalid value "<v>" for <name> (expected an integer)`; also `a non-negative integer`, `a number`, `a duration such as "1m30s"` |

In run-time messages `<name>` is the option as declared (`--heat`, `-k`) or
the argument as `<dish>`. Positional argument errors are prefixed with the
command's full name; flag errors are wrapped by urfave/cli in
`invalid value "<v>" for flag -<name>: ...`.

## Usage lines

These are used by the Markdown and man outputs and by urfave/cli's
`ArgsUsage`.

- Positional arguments render as `<name>`, with `...` when the maximum is
  above 1, in `[...]` when the argument is optional: `<input> [<extra>...]`.
  Hidden arguments are left out.
- A command's synopsis is its full name, then `[options]` when it has or
  inherits any option, then `<command>` for a command that only dispatches,
  or `[<command>]` for a runnable command with subcommands, then the
  arguments: `tool config [options] [<command>] [<key>] [<value>...]`.
- `x-usage`, when present, replaces the synopsis in all outputs.
- An option's value placeholder is `<name>` for each of its arguments,
  separated by spaces, with `...` after a single argument that takes several
  values: `-I <dir>...`.

## Value details in documentation

Options and arguments list, in this order and separated by `; `, the parts
that apply: the type when `x-type` declares one, `one of: a, b` for accepted
values, the range, and `default: ...`. Ranges read `1..9`, `>= 1` or
`<= 9`. Several default values are joined with `, `. Example:
`int; >= 1; default: 1`.

## Positional arguments at run time

Generated urfave/cli code checks positional arguments before calling the
handler:

1. The total number of values must lie between the sum of the arguments'
   minimums and the sum of their maximums; otherwise the error is
   `<command>: expected at least N argument(s), got M` (or `at most`).
2. Values are assigned to arguments in order. Each argument takes as many
   values as it can, up to its maximum, while leaving enough values for the
   minimums of the arguments after it. With `[<a>...] <b>` and two values,
   `<a>` gets the first and `<b>` the second.
3. Each value is checked against its argument's type, accepted values and
   range. The error names the command and the argument, for example
   `kitchen cook: invalid value "rice" for <dish> (accepted: pasta, soup, salad)`.

## urfave/cli output

### Commands and handlers

- The `Handlers` interface has one method per runnable command, listed
  parents first, in the order of the description.
- The method name is `x-go-name` when set (it must be an exported Go
  identifier). Otherwise it is the command path without the root, each word
  capitalised and everything that is not a letter or digit removed:
  `remote set-url` becomes `RemoteSetUrl`; a name starting with a digit gets
  the prefix `Cmd`; the root is `Root`. Two commands with the same method
  name are an error.
- A command's `Usage` is its summary. Its `Description` is, separated by
  blank lines: the description without the summary line, "This command is
  interactive." for interactive commands, `Examples:` with one example per
  line, `Exit codes:` with one `code  description` per line, and
  `Documentation: <url>`.
- `x-category` becomes the command's category, and the root gets
  `UseShortOptionHandling` when `conventions.groupOptions` is true (the
  default).

### Flags

- Options named `help` are never generated, nor the root's `version` option
  when `info.version` is set: urfave/cli provides both. Other options may use
  `-h` or `-v`; urfave/cli then drops those short aliases from its own flags.
- Non-recursive options are `Local`: they apply to their own command only,
  and are rejected after a subcommand name.
- Supported option separators are `" "` and `"="`; urfave/cli accepts both
  forms whichever is declared.
- Problems urfave/cli cannot express are errors, or warnings with
  `--lenient`: other option separators; options with more than one argument
  per use (lenient: a repeatable flag); a subcommand option that reuses some
  names of an inherited recursive option, because urfave/cli would drop the
  whole inherited option in that subcommand (lenient: generated as is).
- The shadowing check skips options urfave/cli provides itself: a
  subcommand option named `help` (or the root's `version`) is not generated,
  and an inherited `help` or `version` option may lose a short name such as
  `-h` to a subcommand option, which urfave/cli handles. This is what lets
  git's `grep -h` coexist with the inherited `--help`/`-h`.

### Generated file

- The file depends only on the standard library and urfave/cli: the argument
  and value checks are copied into it from `internal/gen/urfave/runtime` as
  unexported functions prefixed `ocli`.
- It starts with `// Code generated by opencligen from <source>. DO NOT EDIT.`,
  where the source is the description's path as given, or `--source`, then
  the package clause and imports (the standard library packages the helpers
  use, `context`, and urfave/cli).
- The `Handlers` interface is documented as `// Handlers implements the
  commands described in <source>.`, each method as `// <Method> runs
  `<command path>`.`, and `New` as `// New returns the <root> command tree
  wired to h.`
- After `New` comes the comment `// The helpers below are copied from
  opencligen's internal/gen/urfave/runtime` / `// package so that this file
  depends only on urfave/cli.`, then the helpers verbatim.
- The order of fields inside each `cli.Command` or flag literal is not
  specified; tests check the resulting command tree, not the source text.

## Documentation outputs

- Hidden commands, their subcommands, and hidden options and arguments are
  left out unless `--hidden` is given.
- Markdown: a title (`info.title`, or the root name), the root's summary and
  the rest of its description, a table of version, license, contact and
  documentation link, and a nested index of commands (left out when only one
  command is documented). Then one section per command, headed by its full
  name in code. Other commands' sections start with their summary and the
  rest of their description; the root's does not repeat them. Each section
  has the usage block, then these subsections when they have content, in
  this order: Options, Inherited options, Arguments, Subcommands, Examples,
  Exit codes. Every `|` in a table cell is escaped.
- Man page: `.TH` with the upper-case root name, the section (1 by default),
  an empty date unless `--date` is given, `<name> <version>` and the title;
  then NAME, SYNOPSIS, DESCRIPTION, OPTIONS, ARGUMENTS, COMMANDS (one
  subsection per command), EXAMPLES, EXIT STATUS and SEE ALSO, each only
  when it has content. Paragraphs inside an option, argument or exit status
  entry stay indented under it.
- An output file whose content would not change is not rewritten.

### Markdown details

- The facts table has rows Version, License (`name (identifier)`, linked to
  the license URL), Contact (the name linked to the contact URL, then the
  email as a `mailto:` link) and Documentation (the link).
- A command's notes are the lines of one paragraph, each ended by a hard
  line break, in this order and when they apply: ``Aliases: `c` ``,
  `Category: Meals`, `Interactive: this command prompts for input.`,
  `Hidden.` (only shown with `--hidden`), and `Documentation: <link>` (for
  commands other than the root, whose link is in the facts table).
- Option and argument rows: the value cell is the placeholder in code, a
  space, then the value details; the description cell is `Required. `
  (when required), the description, then ` Group: <group>.` (when grouped).

A command section, from the all-features case:

````markdown
## `kitchen cook`

Cook a dish

Cooks the given dish with the given options.

Aliases: `c`
Category: Meals

```
kitchen cook [options] <dish> [<servings>]
```

### Options

| Option | Value | Description |
|---|---|---|
| `--heat` | `<level>` int; 1..9; default: 5 | Heat level |
| `--style` | `<style>` one of: plain, fancy; default: plain | Group: Presentation. |
| `--weights`, `-w` | `<kg>...` float; >= 0; default: 0.5, 2 |  |
| `--batch` | `<id>` uint | Required. |

### Inherited options

| Option | Value | Description |
|---|---|---|
| `--verbose`, `-v` |  | Print more |

### Arguments

| Argument | Values | Description |
|---|---|---|
| `<dish>` | one of: pasta, soup, salad | Required. |
| `<servings>` | int; 1..12 | How many people |

### Examples

```
kitchen cook pasta 2
```

### Exit codes

| Code | Description |
|---|---|
| 0 | Served |
````

### Man page details

- A subsection starts with the synopsis: an `x-usage` as a literal block
  (`.nf` … `.fi`), otherwise `.B <full name>` followed by the rest of the
  synopsis. Then the description (or the summary when there is no
  description), `Aliases: a, b.`, and `This command prompts for input.` for
  interactive commands, each as its own paragraph.
- Then the labelled lists `Options:`, `Arguments:`, `Examples:` (an indented
  literal block) and `Exit status:`, each when it has content. Subsections
  list a command's own options only: no inherited options and no category.
- Entry bodies are `Required. ` (when required), the description, then the
  value details in parentheses.
- SEE ALSO lists, one per line: `Documentation: <url>`, `Contact: <name>,
  <email>, <url>` (the parts that are set) and `License: <name or
  identifier>`.

The same command, from the all-features case:

```roff
.SS "kitchen cook"
.nf
kitchen cook [options] <dish> [<servings>]
.fi
.PP
Cook a dish
.PP
Cooks the given dish with the given options.
.PP
Aliases: c.
.PP
\fBOptions:\fR
.TP
\fB\-\-heat\fR \fI<level>\fR
Heat level (int; 1..9; default: 5)
.TP
\fB\-\-batch\fR \fI<id>\fR
Required. (uint)
.PP
\fBArguments:\fR
.TP
\fIdish\fR
Required. (one of: pasta, soup, salad)
.PP
\fBExamples:\fR
.PP
.RS 4
.nf
kitchen cook pasta 2
.fi
.RE
.PP
\fBExit status:\fR
.TP
\fB0\fR
Served
```

## The opencligen command

- `validate` prints `<path>: valid OpenCLI <version> description`.
- Diagnostics are printed to standard error, one per line; when there are
  errors, they are followed by `opencligen: <n> error(s)` and the exit
  status is 1.
- Usage errors (unknown flags or commands, wrong argument counts) print
  `opencligen: <error>` and `Run "opencligen --help" for usage.` to standard
  error, with exit status 2.
- No arguments, `help` and `--help` print help and exit 0.
- `--version` prints `opencligen version <version> (OpenCLI <versions>)`,
  where the version is the release tag set at build time, else the module
  version from Go's build information, else `(devel)`.
