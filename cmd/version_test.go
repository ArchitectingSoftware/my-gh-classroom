package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCommandNeedsNoConfig(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	missing := filepath.Join(t.TempDir(), "no-such-config.json")
	if err := run(t, missing, "version"); err != nil {
		t.Fatalf("version should not need a config: %v", err)
	}
	if !strings.HasPrefix(out.String(), "mgc ") || !strings.Contains(out.String(), "go:") {
		t.Errorf("output:\n%s", out.String())
	}
}

func TestVersionStampedAtBuildTime(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v1.2.3"
	if v, _, _, _ := buildVersion(); v != "v1.2.3" {
		t.Errorf("version = %q", v)
	}
	version = ""
	if v, _, _, _ := buildVersion(); v == "" {
		t.Error("unstamped build should report a fallback version")
	}
}

func TestVersionFlag(t *testing.T) {
	if rootCmd.Version == "" {
		t.Error("--version should be enabled")
	}
}
