package schema_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/angrifel/opencligen/internal/opencli/schema"
)

func TestVersions(t *testing.T) {
	if got := schema.Versions(); !slices.Equal(got, []string{"0.1"}) {
		t.Errorf("Versions = %q", got)
	}
}

func TestMajorMinor(t *testing.T) {
	for in, want := range map[string]string{"0.1": "0.1", "0.1.3": "0.1", "1.2.3.4": "1.2", "1": "1", "": ""} {
		if got := schema.MajorMinor(in); got != want {
			t.Errorf("MajorMinor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFor(t *testing.T) {
	s, err := schema.For("0.1")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema   string   `json:"$schema"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(s, &doc); err != nil {
		t.Fatalf("embedded schema is not JSON: %v", err)
	}
	if doc.Schema != "https://json-schema.org/draft/2020-12/schema" || !slices.Contains(doc.Required, "opencli") {
		t.Errorf("unexpected embedded schema: %+v", doc)
	}

	// Patch versions only clarify the specification.
	patch, err := schema.For("0.1.7")
	if err != nil || !bytes.Equal(patch, s) {
		t.Errorf("For(0.1.7) = %d bytes, %v; want the 0.1 schema", len(patch), err)
	}

	for _, v := range []string{"0.2", "1.0", "", "x"} {
		_, err := schema.For(v)
		if err == nil || !strings.Contains(err.Error(), `unsupported OpenCLI version "`+v+`" (supported: 0.1)`) {
			t.Errorf("For(%q): error = %v", v, err)
		}
	}
}
