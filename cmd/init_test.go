package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
)

// runInit runs mgc in a fresh temp working directory and HOME with no
// config file anywhere, returning that directory and HOME.
func runInit(t *testing.T, args ...string) (dir, home string, err error) {
	t.Helper()
	dir, home = t.TempDir(), t.TempDir()
	wd, _ := os.Getwd()
	if e := os.Chdir(dir); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("MGC_CONFIG", "")
	return dir, home, again(t, args...)
}

// again runs mgc once more in the current directory and HOME.
func again(t *testing.T, args ...string) error {
	t.Helper()
	apply, classroomAlias, configPath = false, "", ""
	if c, _, err := rootCmd.Find([]string{"init"}); err == nil {
		_ = c.Flags().Set("home", "false")
	}
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func TestInitDryRunWritesNothing(t *testing.T) {
	dir, _, err := runInit(t, "init")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Error("dry run created config.json")
	}
}

func TestInitWritesLocalConfig(t *testing.T) {
	dir, _, err := runInit(t, "--apply", "init", "cs472")
	if err != nil {
		t.Fatal(err)
	}
	c := load(t, filepath.Join(dir, "config.json"))
	if c.DefaultClassroom != "cs472" {
		t.Errorf("default classroom = %q, want cs472", c.DefaultClassroom)
	}
	if cl := c.Classrooms["cs472"]; cl.Organization != "YOUR_GITHUB_ORGANIZATION" || cl.RepoNameCase != config.RepoNameCaseLower {
		t.Errorf("unexpected classroom: %+v", cl)
	}
}

func TestInitNeverOverwrites(t *testing.T) {
	dir, _, err := runInit(t, "--apply", "init")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "config.json")
	os.WriteFile(p, []byte("mine"), 0o600)
	for _, args := range [][]string{{"init"}, {"--apply", "init", "cs472"}} {
		err := again(t, args...)
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Errorf("%v: err = %v, want 'already exists'", args, err)
		}
	}
	if b, _ := os.ReadFile(p); string(b) != "mine" {
		t.Errorf("existing config.json was modified: %q", b)
	}
}

func TestInitHome(t *testing.T) {
	dir, home, err := runInit(t, "--apply", "init", "--home")
	if err != nil {
		t.Fatal(err)
	}
	load(t, filepath.Join(home, ".mgc", "config.json"))
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Error("--home also wrote ./config.json")
	}
}

func TestInitRejectsConfigAndHome(t *testing.T) {
	_, _, err := runInit(t, "--config", "x.json", "init", "--home")
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("err = %v", err)
	}
}

func TestInitRejectsBadAlias(t *testing.T) {
	_, _, err := runInit(t, "init", "bad alias")
	if err == nil {
		t.Error("expected an error for an invalid alias")
	}
}
