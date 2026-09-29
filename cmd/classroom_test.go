package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
)

const sampleConfig = `{
  "default_classroom": "cs472",
  "classrooms": {
    "cs472": {
      "organization": "CS472-Net-WI26",
      "grader_team": "graders",
      "grader_permission": "push",
      "student_permission": "push",
      "course_info_repo": "course-info",
      "course_info_url": "https://github.com/CS472-Net-WI26/course-info"
    }
  }
}
`

// run executes the CLI in-process against a temp config file. Package-level
// flag variables persist between Cobra executions, so they are reset here.
func run(t *testing.T, cfgFile string, args ...string) error {
	t.Helper()
	apply, classroomAlias, configPath = false, "", ""
	resultsFile = filepath.Join(t.TempDir(), "import-results.txt")
	messagesFile = filepath.Join(t.TempDir(), "messages.txt")
	if err := rejectLegacyArgs(args); err != nil {
		return err
	}
	if c, _, err := rootCmd.Find([]string{"classroom", "import"}); err == nil {
		_ = c.Flags().Set("number", "0")
		_ = c.Flags().Set("name-column", "Name")
		_ = c.Flags().Set("github-column", "GitHub-ID")
		_ = c.Flags().Set("repair", "false")
		_ = c.Flags().Set("message", "false")
	}
	rootCmd.SetArgs(append([]string{"--config", cfgFile}, args...))
	return rootCmd.Execute()
}

func tempConfig(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(sampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func load(t *testing.T, p string) config.Config {
	t.Helper()
	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClassroomCreateDryRunDoesNotWrite(t *testing.T) {
	p := tempConfig(t)
	before, _ := os.ReadFile(p)
	if err := run(t, p, "classroom", "create", "cs281"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Error("dry run modified config")
	}
}

func TestClassroomCreateApply(t *testing.T) {
	p := tempConfig(t)
	if err := run(t, p, "--apply", "classroom", "create", "cs281"); err != nil {
		t.Fatal(err)
	}
	c := load(t, p)
	if c.Classrooms["cs281"].Organization != "YOUR_GITHUB_ORGANIZATION" {
		t.Errorf("classroom not created: %+v", c.Classrooms)
	}
	if c.Classrooms["cs281"].CourseName != "CS281" {
		t.Errorf("course_name = %q, want CS281", c.Classrooms["cs281"].CourseName)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `"repo_prefix": ""`) {
		t.Errorf("new classroom should show repo_prefix:\n%s", data)
	}
	if err := run(t, p, "--apply", "classroom", "create", "cs281"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate create: %v", err)
	}
}

func TestClassroomDefaultAndDelete(t *testing.T) {
	p := tempConfig(t)
	if err := run(t, p, "--apply", "classroom", "create", "cs281"); err != nil {
		t.Fatal(err)
	}
	if err := run(t, p, "classroom", "delete", "cs472"); err == nil {
		t.Error("deleting the default classroom should fail")
	}
	if err := run(t, p, "classroom", "default", "cs281"); err != nil {
		t.Fatal(err)
	}
	if load(t, p).DefaultClassroom != "cs472" {
		t.Error("dry-run default change was persisted")
	}
	if err := run(t, p, "--apply", "classroom", "default", "cs281"); err != nil {
		t.Fatal(err)
	}
	if err := run(t, p, "--apply", "classroom", "delete", "cs472"); err != nil {
		t.Fatal(err)
	}
	c := load(t, p)
	if c.DefaultClassroom != "cs281" || len(c.Classrooms) != 1 {
		t.Errorf("unexpected config: %+v", c)
	}
}

func TestUnknownClassroomErrors(t *testing.T) {
	p := tempConfig(t)
	err := run(t, p, "-c", "nope", "student", "list")
	if err == nil || !strings.Contains(err.Error(), "classroom 'nope' does not exist") {
		t.Fatalf("err = %v", err)
	}
	if err := run(t, p, "classroom", "default", "nope"); err == nil {
		t.Error("expected error setting unknown default")
	}
}

func TestRootCommandName(t *testing.T) {
	if rootCmd.Use != "mgc" {
		t.Errorf("root command Use = %q, want mgc", rootCmd.Use)
	}
}

func TestSingleDashApplyIsRejected(t *testing.T) {
	p := tempConfig(t)
	before, _ := os.ReadFile(p)
	err := run(t, p, "-apply", "classroom", "create", "cs281")
	if err == nil || !strings.Contains(err.Error(), "use --apply or -a") {
		t.Fatalf("err = %v, want a hint to use --apply or -a", err)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Error("-apply must not modify anything")
	}
}

func TestShortApplyFlag(t *testing.T) {
	p := tempConfig(t)
	if err := run(t, p, "-a", "classroom", "create", "cs281"); err != nil {
		t.Fatal(err)
	}
	if _, ok := load(t, p).Classrooms["cs281"]; !ok {
		t.Error("-a did not apply")
	}
}

func TestClassroomFlagForms(t *testing.T) {
	p := tempConfig(t)
	for _, args := range [][]string{
		{"-c", "nope", "student", "list"},
		{"-cnope", "student", "list"},
		{"--classroom", "nope", "student", "list"},
		{"--classroom=nope", "student", "list"},
	} {
		err := run(t, p, args...)
		if err == nil || !strings.Contains(err.Error(), "classroom 'nope' does not exist") {
			t.Errorf("%v: err = %v", args, err)
		}
	}
}

func TestLegacyClassroomFlagIsRejected(t *testing.T) {
	err := run(t, tempConfig(t), "-cr", "cs472", "student", "list")
	if err == nil || !strings.Contains(err.Error(), "use --classroom or -c") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewClassroomHasInstructors(t *testing.T) {
	p := tempConfig(t)
	if err := run(t, p, "--apply", "classroom", "create", "cs281"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `"instructors": []`) {
		t.Errorf("new classroom should show instructors:\n%s", data)
	}
}
