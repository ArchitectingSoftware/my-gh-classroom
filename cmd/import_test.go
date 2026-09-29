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
	if !strings.Contains(err.Error(), "--name-column/--github-column") || !strings.Contains(err.Error(), "see: mgc classroom import --help") {
		t.Errorf("missing-column error should point to --help: %v", err)
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
	err := run(t, tempConfig(t), "-c", "nope", "classroom", "import", "x.csv")
	if err == nil || !strings.Contains(err.Error(), "classroom 'nope' does not exist") {
		t.Fatalf("err = %v", err)
	}
}

func TestImportHelpListsRequiredColumns(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"classroom", "import"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Name", "GitHub-ID", "other columns are ignored", "--name-column", "--github-column", "--repair", "repo_prefix"} {
		if !strings.Contains(c.Long, want) {
			t.Errorf("help text missing %q", want)
		}
	}
}

func TestImportColumnOverridesAreUsed(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "export.csv")
	os.WriteFile(csv, []byte("Student,GitHub Username\nJane Smith,jsmith\n"), 0o600)
	t.Setenv("PATH", "") // columns must be checked before any gh call

	// Wrong override: the error names the column that was asked for.
	err := run(t, tempConfig(t), "classroom", "import", csv, "--name-column", "Student", "--github-column", "Handle")
	if err == nil || !strings.Contains(err.Error(), `"Handle"`) {
		t.Fatalf("err = %v", err)
	}
	// Correct overrides get past the column check and on to GitHub.
	err = run(t, tempConfig(t), "classroom", "import", csv, "--name-column", "Student", "--github-column", "GitHub Username")
	if err == nil || strings.Contains(err.Error(), "missing required columns") || !strings.Contains(err.Error(), "gh") {
		t.Fatalf("err = %v, want a gh error after the columns were accepted", err)
	}
}

func TestCreateBatchRemoved(t *testing.T) {
	c, _, _ := rootCmd.Find([]string{"student", "create-batch"})
	if c != nil && c.Name() == "create-batch" {
		t.Error("student create-batch should be removed")
	}
}

func TestMessageFlagsExist(t *testing.T) {
	for _, path := range [][]string{{"classroom", "import"}, {"student", "invites"}, {"team", "invites"}} {
		c, _, err := rootCmd.Find(path)
		if err != nil || c.Flags().Lookup("message") == nil {
			t.Errorf("%v should have --message (err %v)", path, err)
		}
	}
}

func TestEmitMessagesOverwritesStaleFile(t *testing.T) {
	messagesFile = filepath.Join(t.TempDir(), "messages.txt")
	os.WriteFile(messagesFile, []byte("OLD MESSAGE FROM LAST WEEK"), 0o600)
	if err := emitMessages(nil, "nothing to do"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(messagesFile)
	if strings.Contains(string(data), "OLD MESSAGE") || !strings.Contains(string(data), "nothing to do") {
		t.Errorf("stale messages file not overwritten: %q", data)
	}
}
