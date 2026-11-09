// Command cmd regenerates opencligen's own CLI code and docs; run it from
// the repository root ("make generate").
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/angrifel/opencligen/internal/selfgen"
)

func main() {
	files, err := selfgen.Files(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if old, err := os.ReadFile(name); err == nil && bytes.Equal(old, files[name]) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(name, files[name], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("wrote", name)
	}
}
