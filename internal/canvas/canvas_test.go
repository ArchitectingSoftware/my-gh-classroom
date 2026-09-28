package canvas

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// header matches a real Canvas "Student Analysis" export.
const header = "Name,ID,SectionIDs,SectionNames,Submitted,ElapsedTime,Attempt,ItemID,ItemType,GitHub-ID\n"

func row(name, id string) string {
	return name + ",1,12141,CS-472-002 - FA 26-27,2026-09-23 16:06:29 UTC,0:01:00,1,123456,rich-fill-blank," + id + "\n"
}

func TestReadCanvasExport(t *testing.T) {
	in := header + row("Jane Smith", "jsmith42") + row(" Mary  Ann   Lee ", " @mlee ") + row("Bob Jones", "https://github.com/bjones/")
	got, err := Read(strings.NewReader(in), 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []Student{
		{Row: 2, Name: "Jane Smith", GitHubID: "jsmith42", RawID: "jsmith42"},
		{Row: 3, Name: "Mary Ann Lee", GitHubID: "mlee", RawID: "@mlee"},
		{Row: 4, Name: "Bob Jones", GitHubID: "bjones", RawID: "https://github.com/bjones/"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d students, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("student %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got[0].Normalized() || !got[1].Normalized() {
		t.Error("Normalized() wrong")
	}
}

func TestReadLimit(t *testing.T) {
	in := header + row("A A", "a") + row("B B", "b") + row("C C", "c")
	for limit, want := range map[int]int{0: 3, 1: 1, 2: 2, 3: 3, 10: 3} {
		got, err := Read(strings.NewReader(in), limit)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want {
			t.Errorf("limit %d: got %d records, want %d", limit, len(got), want)
		}
	}
	if _, err := Read(strings.NewReader(in), -1); err == nil {
		t.Error("negative limit should error")
	}
}

func TestReadBOMAndCRLFAndNoTrailingNewline(t *testing.T) {
	in := "\ufeff" + strings.ReplaceAll(header+row("A A", "a"), "\n", "\r\n") + strings.TrimSuffix(row("B B", "b"), "\n")
	got, err := Read(strings.NewReader(in), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].GitHubID != "a" || got[1].GitHubID != "b" {
		t.Errorf("got %+v", got)
	}
}

func TestReadQuotedNameWithComma(t *testing.T) {
	in := header + `"Smith, Jane",1,1,S,t,1,1,1,x,jsmith` + "\n"
	got, err := Read(strings.NewReader(in), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "Smith, Jane" {
		t.Errorf("name = %q", got[0].Name)
	}
}

func TestReadShortRowAndBlankID(t *testing.T) {
	in := header + "Only Name\n" + row("No Id", "")
	got, err := Read(strings.NewReader(in), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].GitHubID != "" || got[1].GitHubID != "" {
		t.Errorf("got %+v", got)
	}
}

func TestReadMissingColumns(t *testing.T) {
	cases := map[string]string{
		"no github":  "Name,ID\nJane,1\n",
		"no name":    "Student,GitHub-ID\nJane,j\n",
		"neither":    "a,b\n1,2\n",
		"wrong case": "name,github-id\nJane,j\n",
	}
	for label, in := range cases {
		_, err := Read(strings.NewReader(in), 0)
		if !errors.Is(err, ErrMissingColumns) {
			t.Errorf("%s: err = %v, want ErrMissingColumns", label, err)
		}
	}
	_, err := Read(strings.NewReader("Name,ID\n"), 0)
	if err == nil || !strings.Contains(err.Error(), `"GitHub-ID"`) || !strings.Contains(err.Error(), "found: Name, ID") {
		t.Errorf("error should name missing and found columns: %v", err)
	}
}

func TestReadEmptyFile(t *testing.T) {
	if _, err := Read(strings.NewReader(""), 0); err == nil {
		t.Error("expected error for empty file")
	}
}

func TestReadHeaderOnly(t *testing.T) {
	got, err := Read(strings.NewReader(header), 0)
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestReadFileMissing(t *testing.T) {
	if _, err := ReadFile(filepath.Join(t.TempDir(), "nope.csv"), 0); err == nil {
		t.Error("expected error")
	}
}

func TestReadFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.csv")
	os.WriteFile(p, []byte(header+row("A A", "a")), 0o600)
	got, err := ReadFile(p, 0)
	if err != nil || len(got) != 1 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestNormalizeGitHubID(t *testing.T) {
	cases := map[string]string{
		"jsmith":                        "jsmith",
		"  jsmith ":                     "jsmith",
		"@jsmith":                       "jsmith",
		"https://github.com/jsmith":     "jsmith",
		"http://www.github.com/jsmith/": "jsmith",
		"github.com/jsmith":             "jsmith",
		"GitHub.com/JSmith":             "JSmith",
		"":                              "",
	}
	for in, want := range cases {
		if got := NormalizeGitHubID(in); got != want {
			t.Errorf("NormalizeGitHubID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidGitHubID(t *testing.T) {
	for _, ok := range []string{"a", "jsmith42", "j-smith", "A1", strings.Repeat("a", 39)} {
		if !ValidGitHubID(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "-js", "j smith", "j_smith", "jsmith@drexel.edu", "12345678 ", strings.Repeat("a", 40)} {
		if ValidGitHubID(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
