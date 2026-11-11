// Package schema embeds the OpenCLI JSON schemas supported by opencligen.
//
// Each supported OpenCLI major.minor version has its own directory holding
// the upstream schema.json, taken unmodified from
// https://github.com/spectreconsole/open-cli (see the Makefile for the pinned
// commit). The schemas are MIT-licensed, Copyright (c) 2025 Patrik Svensson
// and contributors; see LICENSE-open-cli.md in this directory.
package schema

import (
	"embed"
	"fmt"
	"slices"
	"strings"
)

//go:embed */schema.json
var files embed.FS

// Versions returns the supported OpenCLI major.minor versions, oldest first.
func Versions() []string {
	entries, _ := files.ReadDir(".")
	var vs []string
	for _, e := range entries {
		if e.IsDir() {
			vs = append(vs, e.Name())
		}
	}
	slices.Sort(vs)
	return vs
}

// MajorMinor returns the major.minor part of an OpenCLI version such as
// "0.1" or "0.1.2".
func MajorMinor(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}

// For returns the schema for an OpenCLI document version. Patch versions only
// clarify the specification, so "0.1.3" uses the 0.1 schema.
func For(version string) ([]byte, error) {
	mm := MajorMinor(version)
	b, err := files.ReadFile(mm + "/schema.json")
	if err != nil {
		return nil, fmt.Errorf("unsupported OpenCLI version %q (supported: %s)", version, strings.Join(Versions(), ", "))
	}
	return b, nil
}
