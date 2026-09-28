package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportRequiresFile(t *testing.T) {
	err := run(t, tempConfig(t), "classroom", "import")
	if !errors.Is(err, errUsageShown) {
		t.Fatalf("err = %v, want errUsageShown", err)
	}
}

func TestImportRejectsExtraArgs(t *testing.T) {
	err := run(t, tempConfig(t), "classroom", "import", "a.csv", "b.csv")
	if err == nil || !strings.Contains(err.Error(), "expected one CSV file") {
		t.Fatalf("err = %v", err)
	}
}

func TestImportChecksColumnsBeforeContactingGitHub(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "export.csv")
	os.WriteFile(csv, []byte("Name,Email\nJane Smith,j@x.edu\n"), 0o600)
	// Empty PATH: any attempt to run gh would fail with "gh was not found".
	t.Setenv("PATH", "")
	err := run(t, tempConfig(t), "classroom", "import", csv)
	if err == nil || !strings.Contains(err.Error(), `missing required columns "GitHub-ID"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestImportMissingFile(t *testing.T) {
	err := run(t, tempConfig(t), "classroom", "import", filepath.Join(t.TempDir(), "nope.csv"))
	if err == nil || !strings.Contains(err.Error(), "could not open") {
		t.Fatalf("err = %v", err)
	}
}

func TestImportNegativeNumber(t *testing.T) {
	err := run(t, tempConfig(t), "classroom", "import", "x.csv", "-n", "-2")
	if err == nil || !strings.Contains(err.Error(), "--number") {
		t.Fatalf("err = %v", err)
	}
}

func TestImportUsesActiveClassroom(t *testing.T) {
	err := run(t, tempConfig(t), "-cr", "nope", "classroom", "import", "x.csv")
	if err == nil || !strings.Contains(err.Error(), "classroom 'nope' does not exist") {
		t.Fatalf("err = %v", err)
	}
}
