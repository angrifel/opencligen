opencligen - validates OpenCLI descriptions (https://opencli.org/) and generates
urfave/cli v3 Go code, Markdown and man pages from them.

# Specs

All specs are contained in the `specs/` directory; read specs/README.md
first. It holds opencligen's own CLI description (specs/opencligen.json),
the x- metadata vocabulary (specs/extensions.md; keep it in sync with
internal/opencli/parser/value.go and build.go), the generated CLI reference
(specs/cli.md, specs/opencligen.1), the behaviour that is otherwise only in
code (specs/behaviour.md; update it with any change to those rules) and the
golden cases (specs/golden/), which define the expected structure of every
generator's output. When code and specs disagree, the specs win. User-facing overview: README.md.

# Commands

- `make check`: everything a change must pass: `make test`, commit message
  check, `go mod tidy -diff`, `goreleaser check`. The pre-push hook and the
  GitHub Actions workflow (.github/workflows/check.yaml, on every push and
  pull request, with the go.mod Go version and the latest stable) both run
  exactly this; add new checks to `make check`, not to the workflow. The
  workflow is inert until the repository has a GitHub remote.
- `make generate`: regenerate opencligen's own CLI code and docs from
  specs/opencligen.json. Run it after changing the spec or any generator.
  It runs ./internal/selfgen/cmd, which calls the generator packages
  directly, so it works even when the committed generated code is broken.
- `make test`: vet, gofmt check, all tests (includes building generated code).
- `make update-golden`: accept intended changes to generator output; review the
  diff of the .json files in specs/golden before committing.
- `make update-upstream`: refresh internal/opencli/schema/ and
  testdata/upstream/ from
  OPENCLI_REF in the Makefile.

# Layout

- `internal/opencli/schema/<major.minor>/schema.json`: upstream OpenCLI
  schemas, unmodified,
  embedded. One directory per supported version.
- `internal/opencli/ast`: the resolved model (`ast.Model`, commands, options,
  arguments, values), diagnostics, and `ast.Source`, which maps JSON pointers
  to line:col (the role go/token plays for go/ast).
- `internal/opencli/parser`: reads a document, validates it and builds an
  `ast.Model` (`parser.LoadFile`, `parser.Options`). All parsing and
  semantic rules live here.
- `internal/gen/urfave`, `internal/gen/markdown`, `internal/gen/man`:
  outputs; `internal/gen/docs` has shared
  helpers for the documentation outputs.
- `internal/gen/urfave/runtime/`: helpers that the urfave output copies into every
  generated file (see "urfave output" below).
- `internal/selfgen/`: generates opencligen's own CLI files; shared by
  `make generate` and TestGeneratedFilesUpToDate.
- `cmd/opencligen`: the CLI. `internal/app/app.gen.go` is generated from
  `specs/opencligen.json`; `main.go` implements its `Handlers`.
- `specs/`: all specifications and docs (see # Specs). `specs/golden/<case>/`
  holds a golden case: `spec.json` plus `structure.json`, `markdown.json` and
  `man.json`, the expected structure of each output; tests iterate over every
  case directory. specs/golden/README.md explains how
  each is checked. The all-features case (a
  fictional CLI named kitchen) exercises every feature; extend it, or add a
  case, when adding one. Name cases with plain descriptive words, never
  idioms: readers may not have English as their first language.
- `testdata/upstream/`: upstream OpenCLI examples used as a test corpus.

# Decisions

## Architecture

- Everything except the opencligen command is internal:
  opencligen is a tool, not a library. The parser and model live under
  internal/opencli (`ast` holds the model types, `parser` the reading and
  validating, named after go/ast and go/parser; `schema` the embedded
  OpenCLI schemas), and the outputs under internal/gen. Files that also use the standard 
  library's go/ast or go/parser import those as goast and goparser.
- Generators render only from `ast.Model`; they never re-read the JSON. A new
  output is a new package under internal/gen/ that walks the model.
- Generator-specific problems are reported with `Model.Diagnostic`, so they
  carry file:line:col like loader errors.
- Library code takes options structs (`parser.Options`, `urfave.Options`, ...);
  no package-level state.
- Generated code must not depend on opencligen: the urfave
  output embeds its helpers. They live in internal/gen/urfave/runtime as a real,
  tested package; the generator go:embeds runtime.go and copies everything
  after its import block, and the generated file's imports are taken from
  that block plus "context". Rules for runtime.go: unexported identifiers
  prefixed "ocli" (they share the user's package), no imports beyond the
  standard library and urfave/cli. The e2e test builds generated code in a
  module that requires only urfave/cli, which enforces this.

## Strictness

- Errors by default. `Lenient` downgrades only problems that have a sensible
  fallback (untypeable x- metadata becomes untyped; urfave limitations get an
  approximation) and always reports a warning.
- C limit constants (INT_MAX, -FLT_MAX, I64_MIN, ...) are accepted only with
  `AllowCLimits` / `--allow-c-limits`. Plain numeric strings ("12", "2.14748e+09")
  are accepted by default.
- Unknown metadata names are ignored, so documents written for other tools
  still load.
- Documents with an OpenCLI version that has no schema directory are rejected.

## Extensions (see specs/extensions.md)

- `x-type`, `x-default`, `x-range` apply to options and positional arguments.
  The names follow the upstream ffmpeg example; dotnet's `x-value-type` is not
  read. Defaults are real JSON values, but strings are accepted too because
  upstream exporters write everything as text.
- Command metadata `x-category`, `x-usage`, `x-documentation` mirror what the
  upstream examples already use. `x-go-name` is ours and urfave-only.
- OpenCLI allows a subcommand to shadow a name of an inherited recursive option
  (git's `grep -h`), so the parser accepts it. Only the urfave output
  rejects it, because urfave/cli v3 then drops the whole inherited flag in that
  subcommand (verified by experiment).

## urfave output

- Built against urfave/cli v3.14.0. It relies on v3 dropping clashing -h/-v
  aliases from its own help/version flags; re-check when upgrading.
- `--help` (any command) and the root `--version` (when info.version is set)
  are not generated; urfave/cli provides them.
- `conventions.groupOptions` maps to `UseShortOptionHandling` on the root.
  Option separators other than " " and "=" are unsupported.
- Boolean options that take a value ("-flag true") become `StringFlag` with
  `ocliBool`, because `BoolFlag` never consumes an argument.
- Options with more than one argument per use are unsupported (none exist in
  the upstream corpus); lenient mode makes them repeatable flags.
- Handlers: one method per runnable command (leaves, and groups that take
  positional arguments), named by CamelCasing the path without the root, root
  is `Root`, listed parents first.
- The summary is not repeated at the start of Description; docs outputs do the
  same via `docs.Details`.

## Docs outputs

- One Markdown file and one man page for the whole command tree.
- Hidden commands, options and arguments are left out unless `-hidden`.
- The man page has no date unless `-date` is given, so output is reproducible.

## CLI (dogfooded)

- The CLI is built with opencligen's own urfave output: change the command
  line by editing specs/opencligen.json and running `make generate`, never by
  editing app.gen.go. Behaviour goes in main.go's handlers.
- Generated files are committed, so the tool builds without a generation
  step. Regeneration must not need the tool itself: when the generated API
  changes (e.g. removing the clirt import), the committed app.gen.go stops
  compiling and `go run ./cmd/opencligen` cannot fix it. That is why
  `make generate` uses internal/selfgen. TestGeneratedFilesUpToDate fails
  when the committed files are stale.
- main.go adds what the spec cannot say: the version comes from build info
  (info.version in the spec is a placeholder), urfave/cli must never call
  os.Exit (no-op ExitErrHandler), and usage errors are routed through
  OnUsageError so run() reports every error itself.
- Global flags (--lenient, --allow-c-limits) are recursive root options, so
  they work before or after the command. groupOptions is false.
- Running with no arguments prints help and exits 0 (urfave behaviour). 
  exit 2 ("invalid usage", like go, curl, tar) was
  considered and rejected in favour of what docker, gh, cargo and kubectl
  do.
- Exit codes: 0 ok, 1 invalid description or generation error, 2 usage.
  Handlers wrap description and output errors in invalidError; any other
  error (flags, argument counts, unknown commands) is a usage error.
- An output file whose content would not change is not rewritten, so make
  sees it as up to date.

# Testing

- Generated Go code is tested for structure and properties, never as text
  (byte-exact Go goldens broke on harmless reordering).
  internal/gen/urfave/golden_test.go builds the generated code, dumps the
  urfave/cli tree that New returns, checks it against the description
  (every command and option at every depth), runs every runnable command to
  find the handler it reaches, checks that ancestors' recursive options are
  accepted and their non-recursive ones rejected, and compares the dump with
  structure.json. It also runs on the upstream corpus. When the generator
  changes, keep the property checks and wantHandler/wantFlagType in the test
  as an independent statement of the mapping; do not derive them from the
  generator.
- The docs outputs are tested the same way (options M2 and
  R2): internal/gen/markdown and internal/gen/man read the output back into
  an outline
  (outline_test.go, roff_test.go), check it against the description and
  compare it with markdown.json / man.json. The roff reader rejects requests
  and escapes it does not know, so adding one to the generator means
  teaching the reader on purpose. No test compares generated text byte for
  byte, except TestGeneratedFilesUpToDate, which checks that committed
  generated files are current.

- Every user-visible error message has a case in
  internal/opencli/parser/load_test.go or
  internal/gen/urfave/urfave_test.go, including its line:col where relevant.
- internal/gen/urfave/scratch_test.go provides the throwaway module both build tests
  use: it requires only urfave/cli (GOPROXY=off), so generated code that
  depends on opencligen fails to build. internal/gen/urfave/e2e_test.go builds the
  all-features CLI (named kitchen) with hand-written handlers and checks
  runtime behaviour. Both build tests are skipped with -short.
- Upstream corpus expectations (dotnet has real duplicate names; ffmpeg needs
  -lenient and -allow-c-limits, 138 warnings) are asserted in
  internal/opencli/parser/load_test.go. Update them deliberately when
  refreshing the corpus.
- Never json.Unmarshal into a slice that already holds a json.RawMessage of
  the document: it reuses the backing array and corrupts the source bytes
  (a past bug; see TestMultiDefaultDoesNotCorruptDocument).

# Releases

Tools: Cocogitto (`cog`, cog.toml) for versions, CHANGELOG.md
and tags; GoReleaser (.goreleaser.yaml) for archives and the GitHub release.

- Nothing is installed system-wide. scripts/install-tools.sh
  downloads pinned versions into .tools/bin (git-ignored) and checks each
  download against a pinned SHA-256; the Makefile installs a tool on first
  use (`make tools` installs both). Always call them as .tools/bin/cog and
  .tools/bin/goreleaser, never from PATH.
- To upgrade a tool, change its version and every platform's checksum in
  the script. GoReleaser checksums come from its release's checksums.txt;
  Cocogitto publishes none, so its were computed from the downloads.
- GoReleaser is deliberately not a Go tool dependency (`go tool`): it would
  add hundreds of modules to go.mod. Cocogitto is a Rust binary.

- Every commit message follows Conventional Commits: `type(scope): subject`,
  lower-case subject, body as usual. Types used here: feat, fix, docs, test,
  refactor, chore, build, ci, perf. Scopes are package or area names (cli,
  urfave, docs, parser). The subject becomes a CHANGELOG line, so write it
  for users. Commits before this convention were rewritten to follow it.
- `make check-commits` runs `cog check` from the first commit after the empty
  root commit: cog ranges include their first commit, and the root has no
  message. Keep the empty root commit; do not reword it.
- `make hooks` installs a commit-msg hook that runs .tools/bin/cog verify
  and a pre-push hook that runs `make check`;
  the user installs it themselves.
- `make release`: check-commits, then `cog bump --auto`, which runs
  `make test` (pre_bump_hooks), updates CHANGELOG.md, commits
  `chore(version): ...` and tags `vX.Y.Z`. Nothing is pushed.
- `make snapshot`: GoReleaser builds every archive into dist/ without a tag
  or publishing.
- `make publish`: for the tag at HEAD, `cog changelog --at <tag>` becomes the
  release notes (GoReleaser's own changelog is disabled), and GoReleaser
  publishes a draft GitHub release. Needs GITHUB_TOKEN and a remote.
- Releases are drafts (`draft: true`) so the notes and archives get a last
  look on GitHub before the release becomes public and notifies watchers;
  publishing the draft is one click and changes nothing else.
- Changelog entries credit only the commit author: release/changelog.tera is
  Cocogitto's default template without the Co-Authored-By credits. When
  upgrading Cocogitto, compare it with the new default template.
- Release builds set main.buildVersion to the tag via ldflags; --version
  falls back to Go build info, then "(devel)".

# Git

- The default branch is `master`, not `main`.
