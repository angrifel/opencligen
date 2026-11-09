# opencligen

Validate OpenCLI descriptions and generate code and docs from them

opencligen validates OpenCLI descriptions (https://opencli.org/) against the OpenCLI schema and its own rules, and generates a urfave/cli v3 command tree in Go, a Markdown reference or a man page from them.

The x- metadata it understands is described in specs/extensions.md in the opencligen repository.

| | |
|---|---|
| Version | devel |
| Contact | [https://github.com/angrifel/opencligen](https://github.com/angrifel/opencligen) |

## Commands

- [`opencligen`](#opencligen): Validate OpenCLI descriptions and generate code and docs from them
  - [`opencligen validate`](#opencligen-validate): Check a description against the OpenCLI schema and opencligen's rules
  - [`opencligen urfave`](#opencligen-urfave): Generate a github.com/urfave/cli/v3 command tree in Go
  - [`opencligen markdown`](#opencligen-markdown): Generate a Markdown reference
  - [`opencligen man`](#opencligen-man): Generate a roff man page

## `opencligen`

```
opencligen [options] <command>
```

### Options

| Option | Value | Description |
|---|---|---|
| `--lenient` |  | Report problems that can be worked around as warnings instead of errors: x- metadata that cannot be typed, and features the output cannot express. |
| `--allow-c-limits` |  | Accept C limit constants such as INT_MAX, -FLT_MAX or I64_MIN in x-range and x-default. |

### Subcommands

| Command | Description |
|---|---|
| [`validate`](#opencligen-validate) | Check a description against the OpenCLI schema and opencligen's rules |
| [`urfave`](#opencligen-urfave) | Generate a github.com/urfave/cli/v3 command tree in Go |
| [`markdown`](#opencligen-markdown) | Generate a Markdown reference |
| [`man`](#opencligen-man) | Generate a roff man page |

### Exit codes

| Code | Description |
|---|---|
| 0 | Success |
| 1 | The description has errors, or the output cannot be generated |
| 2 | Invalid usage |

## `opencligen validate`

Check a description against the OpenCLI schema and opencligen's rules

```
opencligen validate [options] <spec>
```

### Inherited options

| Option | Value | Description |
|---|---|---|
| `--lenient` |  | Report problems that can be worked around as warnings instead of errors: x- metadata that cannot be typed, and features the output cannot express. |
| `--allow-c-limits` |  | Accept C limit constants such as INT_MAX, -FLT_MAX or I64_MIN in x-range and x-default. |

### Arguments

| Argument | Values | Description |
|---|---|---|
| `<spec>` |  | Required. The OpenCLI description, a JSON file |

### Examples

```
opencligen validate cli.json
opencligen validate --lenient --allow-c-limits ffmpeg.json
```

## `opencligen urfave`

Generate a github.com/urfave/cli/v3 command tree in Go

The generated file declares a Handlers interface with one method per runnable command and a New function that builds the command tree wired to it.

```
opencligen urfave [options] <spec>
```

### Options

| Option | Value | Description |
|---|---|---|
| `--output`, `-o` | `<file>` default: - | Output file, or - for standard output |
| `--package` | `<name>` default: cli | Package name of the generated file |
| `--source` | `<name>` | Document name mentioned in the generated header; defaults to the spec path |

### Inherited options

| Option | Value | Description |
|---|---|---|
| `--lenient` |  | Report problems that can be worked around as warnings instead of errors: x- metadata that cannot be typed, and features the output cannot express. |
| `--allow-c-limits` |  | Accept C limit constants such as INT_MAX, -FLT_MAX or I64_MIN in x-range and x-default. |

### Arguments

| Argument | Values | Description |
|---|---|---|
| `<spec>` |  | Required. The OpenCLI description, a JSON file |

### Examples

```
opencligen urfave -o internal/cli/cli.gen.go --package cli cli.json
```

## `opencligen markdown`

Generate a Markdown reference

```
opencligen markdown [options] <spec>
```

### Options

| Option | Value | Description |
|---|---|---|
| `--output`, `-o` | `<file>` default: - | Output file, or - for standard output |
| `--hidden` |  | Document hidden commands, options and arguments |

### Inherited options

| Option | Value | Description |
|---|---|---|
| `--lenient` |  | Report problems that can be worked around as warnings instead of errors: x- metadata that cannot be typed, and features the output cannot express. |
| `--allow-c-limits` |  | Accept C limit constants such as INT_MAX, -FLT_MAX or I64_MIN in x-range and x-default. |

### Arguments

| Argument | Values | Description |
|---|---|---|
| `<spec>` |  | Required. The OpenCLI description, a JSON file |

### Examples

```
opencligen markdown -o docs/cli.md cli.json
```

## `opencligen man`

Generate a roff man page

```
opencligen man [options] <spec>
```

### Options

| Option | Value | Description |
|---|---|---|
| `--output`, `-o` | `<file>` default: - | Output file, or - for standard output |
| `--section` | `<n>` default: 1 | Manual section |
| `--date` | `<date>` | Date shown in the page footer; empty by default for reproducible output |
| `--hidden` |  | Document hidden commands, options and arguments |

### Inherited options

| Option | Value | Description |
|---|---|---|
| `--lenient` |  | Report problems that can be worked around as warnings instead of errors: x- metadata that cannot be typed, and features the output cannot express. |
| `--allow-c-limits` |  | Accept C limit constants such as INT_MAX, -FLT_MAX or I64_MIN in x-range and x-default. |

### Arguments

| Argument | Values | Description |
|---|---|---|
| `<spec>` |  | Required. The OpenCLI description, a JSON file |

### Examples

```
opencligen man -o docs/mytool.1 cli.json
man -l docs/mytool.1
```

