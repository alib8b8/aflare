package workflow

import (
	"os"
	"testing"
)

func FuzzParseWorkflowString(f *testing.F) {
	f.Add("name: test\nsteps:\n  - node: test\n")
	f.Add("not: [ valid yaml :::")
	f.Add("")
	f.Add("name: \"test\"\ndescription: \"desc\"\nsteps:\n  - node: test\n    params:\n      key: value\n")
	f.Add("steps:\n  - node: test\n")
	f.Add("name: test\n")

	tmpDir, err := os.MkdirTemp("", "fuzz-parser-*")
	if err != nil {
		f.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	f.Fuzz(func(t *testing.T, input string) {
		tmpFile, err := os.CreateTemp(tmpDir, "fuzz-*.yaml")
		if err != nil {
			t.Skipf("failed to create temp file: %v", err)
			return
		}
		path := tmpFile.Name()

		if _, err := tmpFile.WriteString(input); err != nil {
			tmpFile.Close()
			t.Skipf("failed to write temp file: %v", err)
			return
		}
		tmpFile.Close()
		defer os.Remove(path)

		wf, err := ParseWorkflow(path)
		if err != nil {
			return
		}
		if wf == nil {
			t.Errorf("ParseWorkflow returned nil workflow with no error")
		}
	})
}
