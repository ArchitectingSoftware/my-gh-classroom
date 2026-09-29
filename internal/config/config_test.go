package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validClassroom() Classroom {
	return Classroom{
		Organization:      "CS281-Arch-FA26",
		GraderTeam:        "graders",
		GraderPermission:  "push",
		StudentPermission: "push",
		CourseInfoRepo:    "course-info",
		CourseInfoURL:     "https://github.com/CS281-Arch-FA26/course-info",
	}
}

func TestValidateClassroom(t *testing.T) {
	cases := []struct {
		name    string
		alias   string
		mutate  func(*Classroom)
		wantErr string
	}{
		{"valid", "cs281", func(*Classroom) {}, ""},
		{"empty permissions use defaults", "cs281", func(c *Classroom) { c.GraderPermission, c.StudentPermission = "", "" }, ""},
		{"empty course-info repo uses default", "cs281", func(c *Classroom) { c.CourseInfoRepo = "" }, ""},
		{"bad alias", "-cs281", func(*Classroom) {}, "invalid classroom alias"},
		{"alias with space", "cs 281", func(*Classroom) {}, "invalid classroom alias"},
		{"missing org", "cs281", func(c *Classroom) { c.Organization = " " }, `"organization"`},
		{"missing team", "cs281", func(c *Classroom) { c.GraderTeam = "" }, `"grader_team"`},
		{"bad repo", "cs281", func(c *Classroom) { c.CourseInfoRepo = "course info" }, "invalid course_info_repo"},
		{"bad permission", "cs281", func(c *Classroom) { c.StudentPermission = "write" }, "permissions must be"},
		{"http url", "cs281", func(c *Classroom) { c.CourseInfoURL = "http://github.com/x" }, "invalid course_info_url"},
		{"repo prefix", "cs281", func(c *Classroom) { c.RepoPrefix = "cs281-fa26_" }, ""},
		{"repo prefix with space", "cs281", func(c *Classroom) { c.RepoPrefix = "cs 281-" }, "invalid repo_prefix"},
		{"repo prefix with slash", "cs281", func(c *Classroom) { c.RepoPrefix = "cs281/" }, "invalid repo_prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validClassroom()
			tc.mutate(&c)
			err := ValidateClassroom(tc.alias, c)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateConfigDefaultMustExist(t *testing.T) {
	c := Config{DefaultClassroom: "cs472", Classrooms: map[string]Classroom{"cs281": validClassroom()}}
	if err := ValidateConfig(c); err == nil {
		t.Fatal("expected error for missing default")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "config.json")
	cl := validClassroom()
	cl.CourseName = "CS 281"
	cl.RepoPrefix = "cs281-"
	in := Config{DefaultClassroom: "cs281", Classrooms: map[string]Classroom{"cs281": cl}}
	if err := Save(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if out.DefaultClassroom != "cs281" || out.Classrooms["cs281"] != cl {
		t.Errorf("round trip mismatch: %+v", out)
	}
}

func TestSaveAlwaysWritesOptionalFields(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := Save(p, Config{Classrooms: map[string]Classroom{"cs281": validClassroom()}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	for _, k := range []string{`"course_name": ""`, `"repo_prefix": ""`} {
		if !strings.Contains(string(data), k) {
			t.Errorf("config should always contain %s so the setting is discoverable:\n%s", k, data)
		}
	}
}

func TestBlankOptionalFieldsSurviveRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, []byte(`{"classrooms":{"cs281":{"organization":"o","grader_team":"g","course_info_repo":"r","course_info_url":"https://x/y","course_name":"","repo_prefix":""}}}`), 0o600)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `"repo_prefix": ""`) {
		t.Errorf("blank repo_prefix was dropped on save:\n%s", data)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil || !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, []byte(`{"classrooms":{"cs281":{"organization":""}}}`), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestClassroomWithAlias(t *testing.T) {
	named := validClassroom()
	named.CourseName = "Systems Architecture"
	c := Config{
		DefaultClassroom: "cs281",
		Classrooms:       map[string]Classroom{"cs281": validClassroom(), "arch": named},
	}

	alias, cl, err := c.ClassroomWithAlias("")
	if err != nil || alias != "cs281" || cl.CourseName != "CS281" {
		t.Errorf("default: got (%q, %q, %v)", alias, cl.CourseName, err)
	}
	_, cl, _ = c.ClassroomWithAlias("arch")
	if cl.CourseName != "Systems Architecture" {
		t.Errorf("explicit course_name not kept: %q", cl.CourseName)
	}
	if _, _, err := c.ClassroomWithAlias("missing"); err == nil {
		t.Error("expected error for unknown alias")
	}
	if _, _, err := (Config{}).ClassroomWithAlias(""); err == nil {
		t.Error("expected error with no default")
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("HOME", filepath.Join(dir, "home"))

	t.Setenv("MGC_CONFIG", "")
	if got := ResolvePath("/explicit.json"); got != "/explicit.json" {
		t.Errorf("explicit: %q", got)
	}
	if got := ResolvePath(""); !strings.HasSuffix(got, filepath.Join("mgc", "config.json")) {
		t.Errorf("user config dir fallback: %q", got)
	}

	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o600)
	if got := ResolvePath(""); got != "config.json" {
		t.Errorf("local config.json: %q", got)
	}

	t.Setenv("MGC_CONFIG", "/env.json")
	if got := ResolvePath(""); got != "/env.json" {
		t.Errorf("env: %q", got)
	}
}
