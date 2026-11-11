# Behaviour

Rules that opencligen follows and that are not visible from the OpenCLI
specification or [extensions.md](extensions.md). The golden cases
([golden/](golden/)) show them on concrete input; this page states them in
general.

## Reading a description

### Arity and values

- An argument without `arity` takes exactly one value (minimum 1,
  maximum 1, the schema defaults).
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
  command itself or by a nearer ancestor replaces the farther one: it is
  inherited once, from the nearest declaration.
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
  errors. Warnings never change the exit code.

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

### Generated file

- The file depends only on the standard library and urfave/cli: the argument
  and value checks are copied into it from `internal/gen/urfave/runtime` as
  unexported functions prefixed `ocli`.
- It starts with `// Code generated by opencligen from <source>. DO NOT EDIT.`,
  where the source is the description's path as given, or `--source`.

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
