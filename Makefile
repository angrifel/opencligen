# Commit of https://github.com/spectreconsole/open-cli that internal/opencli/schema/ and
# testdata/upstream/ were taken from.
OPENCLI_REF := e42891559d59e84b1da8a8901450fa37784597ed
OPENCLI_RAW := https://raw.githubusercontent.com/spectreconsole/open-cli/$(OPENCLI_REF)

UPSTREAM_EXAMPLES := git curl dotnet ffmpeg

.PHONY: test vet check generate update-golden update-upstream tools check-commits hooks release snapshot publish

## generate: regenerate opencligen's own CLI code and docs from specs/opencligen.json
# Uses the generator packages directly (internal/selfgen), not the opencligen
# command, so it works even when the committed generated code does not build.
generate:
	go run ./internal/selfgen/cmd

## test: vet and run all tests, including building and running generated code
test: vet
	go test ./...

## check: everything a change must pass; run by the pre-push hook and by CI
check: test check-commits $(GORELEASER)
	go mod tidy -diff
	$(GORELEASER) check --quiet

vet:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }

## update-golden: rewrite the expected outputs in specs/golden from the current generators
update-golden:
	go test ./internal/gen/urfave ./internal/gen/markdown ./internal/gen/man -run TestGoldenCases -update

## update-upstream: refresh the embedded schema and the example corpus from OPENCLI_REF
update-upstream:
	curl -fsSL $(OPENCLI_RAW)/schema.json -o internal/opencli/schema/0.1/schema.json
	for e in $(UPSTREAM_EXAMPLES); do \
		curl -fsSL $(OPENCLI_RAW)/examples/$$e.json -o testdata/upstream/$$e.json || exit 1; \
	done

# Releases: Cocogitto (cog.toml) for versions, CHANGELOG.md and tags;
# GoReleaser (.goreleaser.yaml) for archives and the GitHub release. Both are
# pinned and installed into .tools/bin by scripts/install-tools.sh on first
# use; nothing is installed system-wide.
TOOLS      := .tools/bin
COG        := $(TOOLS)/cog
GORELEASER := $(TOOLS)/goreleaser

$(COG) $(GORELEASER): scripts/install-tools.sh
	scripts/install-tools.sh $(notdir $@) && touch $@

## tools: install the pinned release tools into .tools/bin
tools: $(COG) $(GORELEASER)

## check-commits: check that commit messages follow Conventional Commits
# The range starts at the first commit after the empty root commit, which has
# no message (cog ranges include their first commit).
check-commits: $(COG)
	$(COG) check $$(git rev-list --reverse HEAD | sed -n 2p)..HEAD

## hooks: install the git hooks: commit messages are checked on commit, make check runs on push
hooks: $(COG)
	$(COG) install-hook --all

## release: pick the next version from the commits, update CHANGELOG.md, commit and tag (runs make test first)
release: check-commits
	$(COG) bump --auto

## snapshot: build the release archives into dist/ without tagging or publishing
snapshot: $(GORELEASER)
	$(GORELEASER) release --snapshot --clean

## publish: publish the release for the tag at HEAD as a GitHub draft (needs GITHUB_TOKEN; publishing is on hold)
publish: $(COG) $(GORELEASER)
	tag=$$(git describe --tags --exact-match) && \
	$(COG) changelog --at $$tag > .release-notes.md && \
	$(GORELEASER) release --clean --release-notes .release-notes.md
