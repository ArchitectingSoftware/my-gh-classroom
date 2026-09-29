package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
		{"repo name case lower", "cs281", func(c *Classroom) { c.RepoNameCase = "lower" }, ""},
		{"repo name case preserve", "cs281", func(c *Classroom) { c.RepoNameCase = "preserve" }, ""},
		{"repo name case invalid", "cs281", func(c *Classroom) { c.RepoNameCase = "upper" }, "invalid repo_name_case"},
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
	cl.RepoNameCase = RepoNameCasePreserve
	cl.Instructors = []string{"Prof. Mitchell", "Prof. Jones"}
	in := Config{DefaultClassroom: "cs281", Classrooms: map[string]Classroom{"cs281": cl}}
	if err := Save(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if out.DefaultClassroom != "cs281" || !reflect.DeepEqual(out.Classrooms["cs281"], cl) {
		t.Errorf("round trip mismatch: %+v", out)
	}
}

func TestSaveAlwaysWritesOptionalFields(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := Save(p, Config{Classrooms: map[string]Classroom{"cs281": validClassroom()}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	for _, k := range []string{`"course_name": ""`, `"repo_prefix": ""`, `"repo_name_case": "lower"`, `"instructors": []`} {
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
	home := filepath.Join(dir, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("MGC_CONFIG", "")
	homeCfg := filepath.Join(home, ".mgc", "config.json")
	osCfg, _ := os.UserConfigDir()
	osCfg = filepath.Join(osCfg, "mgc", "config.json")
	write := func(p string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if got := ResolvePath("/explicit.json"); got != "/explicit.json" {
		t.Errorf("explicit: %q", got)
	}
	if got := ResolvePath(""); got != homeCfg {
		t.Errorf("nothing exists: got %q, want %q", got, homeCfg)
	}
	write(osCfg)
	if got := ResolvePath(""); got != osCfg {
		t.Errorf("OS config dir: got %q, want %q", got, osCfg)
	}
	write(homeCfg)
	if got := ResolvePath(""); got != homeCfg {
		t.Errorf("~/.mgc should beat the OS config dir: got %q", got)
	}
	write(filepath.Join(dir, "config.json"))
	if got := ResolvePath(""); got != "config.json" {
		t.Errorf("./config.json should beat ~/.mgc: got %q", got)
	}
	t.Setenv("MGC_CONFIG", "/env.json")
	if got := ResolvePath(""); got != "/env.json" {
		t.Errorf("env: %q", got)
	}
}

func TestCreateNeverOverwrites(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := Create(p, Scaffold("cs472")); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatalf("scaffold should load and validate: %v", err)
	}
	if c.DefaultClassroom != "cs472" || c.Classrooms["cs472"].CourseName != "CS472" {
		t.Errorf("unexpected scaffold: %+v", c)
	}
	before, _ := os.ReadFile(p)
	if err := Create(p, Scaffold("other")); !errors.Is(err, ErrExists) {
		t.Fatalf("second Create: err = %v, want ErrExists", err)
	}
	if after, _ := os.ReadFile(p); string(after) != string(before) {
		t.Error("existing file was modified")
	}
}

// config.example.json must be exactly what `mgc init` generates.
func TestExampleConfigMatchesScaffold(t *testing.T) {
	want, err := Encode(Scaffold(ExampleAlias))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "..", "config.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("config.example.json is out of date; regenerate it with:\n  rm config.example.json && go run . --apply --config config.example.json init\nwant:\n%s", want)
	}
}

func TestNormalizeRepoPrefix(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"cs472":   "cs472-",
		"cs472-":  "cs472-",
		"cs472_":  "cs472_",
		"cs472.":  "cs472.",
		" cs472 ": "cs472-",
		"cs-472":  "cs-472-",
	}
	for in, want := range cases {
		if got := NormalizeRepoPrefix(in); got != want {
			t.Errorf("NormalizeRepoPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRepoNameCaseDefaultsToLower(t *testing.T) {
	c := Config{DefaultClassroom: "cs281", Classrooms: map[string]Classroom{"cs281": validClassroom()}}
	cl, err := c.Classroom("")
	if err != nil {
		t.Fatal(err)
	}
	if cl.RepoNameCase != RepoNameCaseLower {
		t.Errorf("RepoNameCase = %q, want %q", cl.RepoNameCase, RepoNameCaseLower)
	}
}

func TestSaveDoesNotModifyCaller(t *testing.T) {
	in := Config{Classrooms: map[string]Classroom{"cs281": validClassroom()}}
	if err := Save(filepath.Join(t.TempDir(), "config.json"), in); err != nil {
		t.Fatal(err)
	}
	if got := in.Classrooms["cs281"]; got.RepoNameCase != "" || got.Instructors != nil {
		t.Errorf("Save modified the caller's classroom: %+v", got)
	}
}
