# OpenCLI extensions understood by opencligen

OpenCLI 0.1 describes commands, options and arguments, but not the types of
values, their defaults or ranges. opencligen reads that information from
`metadata` entries whose names start with `x-`, the convention the upstream
OpenCLI examples use for extensions. Other metadata is ignored, so a
document stays valid OpenCLI and other tools can read it unchanged.

Unless noted otherwise, a problem in an extension is an error. With
`--lenient` (`parser.Options.Lenient`), problems in `x-type`, `x-default` and
`x-range` become warnings and the option or argument is treated as untyped
instead.

## Option and argument values

These entries go in the `metadata` of an option or a positional argument.

```json
{
  "name": "-k",
  "arguments": [{ "name": "n", "required": true }],
  "metadata": [
    { "name": "x-type", "value": "int" },
    { "name": "x-default", "value": 1 },
    { "name": "x-range", "value": { "minimum": 1 } }
  ]
}
```

### `x-type`

The type of the value: a string, one of

| Type | Values | Aliases |
|---|---|---|
| `string` | any text | |
| `int` | signed integers | `int64` |
| `uint` | non-negative integers | `unsigned`, `uint64` |
| `float` | floating point numbers | `double` |
| `boolean` | `true` or `false` (anything Go's `strconv.ParseBool` accepts) | `bool` |
| `duration` | Go durations such as `1m30s` or `250ms` | |

Without `x-type`, an option without arguments is a `boolean` flag that is
either present or absent, and every other value is a `string`.

Rules:

- A type other than `boolean` needs the option to take an argument.
- A `boolean` option that takes an argument is given an explicit value, as in
  `-flag true`.
- `acceptedValues` can only be combined with `string`.
- An option or argument that takes several values (an option with more than
  one argument, or an arity maximum above 1) can be `string`, `int`, `uint` or
  `float`, but not `boolean` or `duration`.

### `x-default`

The value used when the option or argument is not given. It must be valid for
the type, be one of `acceptedValues` when they are given, and lie within
`x-range`.

- For several values, the default is a JSON array, for example `[0.5, 2]`.
- Numbers and booleans may be JSON values (`1`, `true`) or strings (`"1"`,
  `"true"`), as written by tools that export every value as text.
- Durations are strings such as `"1m30s"`.
- Integer types accept integral float notation such as `"2.14748e+09"`.

The urfave/cli output applies option defaults; positional argument defaults
are documented but not applied, since urfave/cli has no place for them.

### `x-range`

An object with a `minimum` and/or a `maximum`, both inclusive, for `int`,
`uint` and `float` values only. Omit a bound to leave that side unbounded.
Bounds are numbers or numeric strings and follow the same rules as numeric
defaults.

#### C limit constants

With `--allow-c-limits` (`parser.Options.AllowCLimits`), bounds and numeric
defaults may also be C limit constants, optionally negated, as in the
upstream ffmpeg example:

| Constant | Value |
|---|---|
| `INT_MIN`, `INT32_MIN` | -2147483648 |
| `INT_MAX`, `INT32_MAX` | 2147483647 |
| `UINT_MAX`, `UINT32_MAX` | 4294967295 |
| `I64_MIN`, `INT64_MIN` | -9223372036854775808 |
| `I64_MAX`, `INT64_MAX` | 9223372036854775807 |
| `UINT64_MAX` | 18446744073709551615 |
| `FLT_MIN`, `FLT_MAX` | 1.1754944e-38, 3.4028235e+38 |
| `DBL_MIN`, `DBL_MAX` | 2.2250738585072014e-308, 1.7976931348623157e+308 |

They are off by default because they tie a description to C's type sizes.

## Command metadata

These entries go in the `metadata` of a command.

| Name | Value | Used for |
|---|---|---|
| `x-category` | string | Groups the command in help output and docs. Used by the upstream git example. |
| `x-usage` | string, or array of strings | Replaces the generated usage line(s). Used by all upstream examples. |
| `x-documentation` | string (URL) | Links to further documentation. |
| `x-go-name` | string | urfave/cli output only: the `Handlers` method name for the command, an exported Go identifier. Use it when the default name reads badly or two commands map to the same name. |

The default handler name is the command path without the root, in
CamelCase: `sheila generate-parser ll` becomes `GenerateParserLl`, and the
root command is `Root`.

## Compatibility with other tools' metadata

opencligen gives `x-type`, `x-default` and `x-range` the meaning above.
Descriptions written for other tools may use the same names differently; the
upstream ffmpeg example, for instance, uses types such as `flags` or
`dictionary` and named defaults such as `"auto"`. Use `--lenient` to load such
documents: those options become untyped, with a warning for each. Metadata
with other names, such as dotnet's `x-value-type`, is ignored.

## urfave/cli limitations

Some OpenCLI features cannot be expressed with urfave/cli v3. The urfave
output reports them as errors, or as warnings with `--lenient`, in which case
it generates the closest approximation:

| Feature | Lenient approximation |
|---|---|
| `conventions.optionSeparator` other than `" "` or `"="` | ignored; urfave/cli accepts `--name value` and `--name=value` |
| An option with more than one argument per use | a repeatable flag |
| A subcommand option reusing a name of an inherited recursive option (urfave/cli would drop the whole inherited option in that subcommand) | generated as is |

`--help` is always provided by urfave/cli, as is `--version` on the root
when `info.version` is set, so options with those names are not generated.
Other options may still use `-h` or `-v`; urfave/cli then drops those short
aliases from its own flags.
