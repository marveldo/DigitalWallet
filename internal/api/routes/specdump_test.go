package routes

import (
	"os"
	"testing"
)

func TestDumpSpec(t *testing.T) {
	out := os.Getenv("DUMP_SPEC")
	if out == "" {
		t.Skip("set DUMP_SPEC to write the generated document to a file")
	}
	doc, err := GenerateDocs(&GenerateDocsConfig{}).MarshalYAML()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, doc, 0o644); err != nil {
		t.Fatal(err)
	}
}
