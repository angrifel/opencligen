# Commit of https://github.com/spectreconsole/open-cli that schema/ and
# testdata/upstream/ were taken from.
OPENCLI_REF := e42891559d59e84b1da8a8901450fa37784597ed
OPENCLI_RAW := https://raw.githubusercontent.com/spectreconsole/open-cli/$(OPENCLI_REF)

UPSTREAM_EXAMPLES := git curl dotnet ffmpeg

.PHONY: test vet generate update-golden update-upstream

## generate: regenerate opencligen's own CLI code and docs from specs/opencligen.json
# Uses the generator packages directly (internal/selfgen), not the opencligen
# command, so it works even when the committed generated code does not build.
generate:
	go run ./internal/selfgen/cmd

## test: vet and run all tests, including building and running generated code
test: vet
	go test ./...

vet:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }

## update-golden: rewrite the expected outputs in specs/golden from the current generators
update-golden:
	go test ./gen/urfave -run TestStructure -update
	go test ./gen/markdown ./gen/man -run TestGolden -update

## update-upstream: refresh the embedded schema and the example corpus from OPENCLI_REF
update-upstream:
	curl -fsSL $(OPENCLI_RAW)/schema.json -o schema/0.1/schema.json
	for e in $(UPSTREAM_EXAMPLES); do \
		curl -fsSL $(OPENCLI_RAW)/examples/$$e.json -o testdata/upstream/$$e.json || exit 1; \
	done
