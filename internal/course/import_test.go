package course

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/canvas"
)

const fullSchema = `[{"property_name":"repo_type"},{"property_name":"student_name"},{"property_name":"github_id"}]`

func importMeta() ImportMeta {
	return ImportMeta{File: "Canvas-Export.csv", Classroom: "cs281", Started: time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)}
}

func stu(row int, name, id string) canvas.Student {
	return canvas.Student{Row: row, Name: name, GitHubID: canvas.NormalizeGitHubID(id), RawID: id}
}

// importFixture wires the preflight calls every import makes.
func importFixture(f *fakeGH, repos string) *fakeGH {
	return f.team().on("GET", "orgs/"+org+"/properties/schema", fullSchema).repos(repos)
}

func newRepoRoutes(f *fakeGH, login string) *fakeGH {
	return f.
		on("POST", "orgs/"+org+"/repos", `{}`).
		on("GET", repoPath(login, "contents/README.md"), `{"sha":"s"}`).
		on("PUT", repoPath(login, "contents/README.md"), `{}`).
		on("PUT", teamRepoPath(login), ``).
		on("PUT", repoPath(login, "collaborators/"+login), `{}`).
		on("PATCH", repoPath(login, "properties/values"), ``)
}

const existingRepos = `[
 {"name":"course-info","html_url":"u/course-info"},
 {"name":"custom-name","html_url":"u/custom-name","custom_properties":{"repo_type":"student","github_id":"bob2","student_name":"Bob"}},
 {"name":"carol3","html_url":"u/carol3"},
 {"name":"taken","html_url":"u/taken","custom_properties":{"repo_type":"student","github_id":"someoneelse"}}
]`

func TestImportDryRunMakesNoChanges(t *testing.T) {
	s, f, out := newService(t, false)
	importFixture(f, existingRepos).user("alice1")

	var results bytes.Buffer
	sum, err := s.ImportStudents([]canvas.Student{
		stu(2, "Alice Anders", "alice1"),
		stu(3, "Bob Baker", "bob2"),
	}, importMeta(), &results)
	if err != nil {
		t.Fatal(err)
	}
	if m := f.mutations(); len(m) != 0 {
		t.Fatalf("dry run performed mutations: %v", m)
	}
	if sum != (ImportSummary{Processed: 2, Created: 1, Skipped: 1}) {
		t.Errorf("summary = %+v", sum)
	}
	o := out.String()
	for _, want := range []string{
		"Mode:         DRY RUN",
		"Alice Anders", "WOULD CREATE", "would create " + org + "/alice1",
		"Bob Baker", "SKIPPING", "repository exists: u/custom-name",
		"Would create: 1", "Dry run only",
	} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
	if results.String() != o {
		t.Error("results file should contain the same report as the screen")
	}
}

func TestImportApplyCreatesOnlyMissing(t *testing.T) {
	s, f, out := newService(t, true)
	newRepoRoutes(importFixture(f, existingRepos).user("alice1"), "alice1")

	sum, err := s.ImportStudents([]canvas.Student{
		stu(2, "Alice Anders", "alice1"),
		stu(3, "Bob Baker", "BOB2"),    // existing, custom repo name, different case
		stu(4, "Carol Chen", "carol3"), // existing by repo name, no properties
	}, importMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum != (ImportSummary{Processed: 3, Created: 1, Skipped: 2}) {
		t.Errorf("summary = %+v", sum)
	}
	for _, m := range f.mutations() {
		if strings.Contains(m, "custom-name") || strings.Contains(m, "carol3") {
			t.Errorf("existing repository was modified: %s", m)
		}
	}
	if len(f.mutations()) != 5 {
		t.Errorf("expected 5 mutations for one new repo, got %v", f.mutations())
	}
	o := out.String()
	for _, want := range []string{"CREATING", "SUCCESS  https://github.com/" + org + "/alice1", "Created:      1",
		`missing custom properties; repair with: mgc -apply student create --name "Carol Chen" --github carol3 --repo carol3`} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
}

func TestImportIsIdempotent(t *testing.T) {
	// Second run over the same roster: everyone now has a repository.
	s, f, _ := newService(t, true)
	importFixture(f, `[{"name":"alice1","html_url":"u/alice1","custom_properties":{"repo_type":"student","github_id":"alice1"}}]`)
	sum, err := s.ImportStudents([]canvas.Student{stu(2, "Alice Anders", "alice1")}, importMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Skipped != 1 || sum.Created != 0 || len(f.mutations()) != 0 {
		t.Errorf("summary %+v, mutations %v", sum, f.mutations())
	}
}

func TestImportPerStudentErrorsDoNotStopTheRun(t *testing.T) {
	s, f, out := newService(t, true)
	importFixture(f, existingRepos).
		fail("GET", "users/ghost", errNotFound).
		user("zed")
	newRepoRoutes(f, "zed")

	sum, err := s.ImportStudents([]canvas.Student{
		stu(2, "Blank Id", ""),
		stu(3, "Bad Id", "jane@drexel.edu"),
		stu(4, "Ghost", "ghost"),
		stu(5, "Taken", "taken"), // repo named "taken" belongs to someone else
		stu(6, "Zed", "zed"),
		stu(7, "Zed Again", "@ZED"), // duplicate of row 6 after normalization
	}, importMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum != (ImportSummary{Processed: 6, Created: 1, Skipped: 1, Errors: 4}) {
		t.Errorf("summary = %+v", sum)
	}
	o := out.String()
	for _, want := range []string{
		"GitHub-ID is blank",
		`GitHub-ID "jane@drexel.edu" is not a valid GitHub username`,
		"GitHub user 'ghost' does not exist",
		"belongs to GitHub user 'someoneelse'",
		"duplicate GitHub-ID, already handled at [5/6]",
		"Errors\n",
		`[3/6] Ghost (GitHub-ID "ghost", CSV row 4)`,
	} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
}

func TestImportPartialFailureIsReportedOnOneLine(t *testing.T) {
	s, f, out := newService(t, true)
	importFixture(f, `[]`).user("alice1").
		on("POST", "orgs/"+org+"/repos", `{}`).
		on("GET", repoPath("alice1", "contents/README.md"), `{"sha":"s"}`).
		fail("PUT", repoPath("alice1", "contents/README.md"), errWithArgs("gh: Server Error (HTTP 500)"))

	sum, err := s.ImportStudents([]canvas.Student{stu(2, "Alice Anders", "alice1")}, importMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Errors != 1 {
		t.Errorf("summary = %+v", sum)
	}
	o := out.String()
	line := ""
	for _, l := range strings.Split(o, "\n") {
		if strings.Contains(l, "CREATING") {
			line = l
		}
	}
	if !strings.Contains(line, "ERROR") || !strings.Contains(line, "repair it with") || !strings.Contains(line, "HTTP 500") {
		t.Errorf("partial failure line = %q", line)
	}
	if strings.Contains(o, "content=") {
		t.Errorf("report leaked gh arguments (base64 content):\n%s", o)
	}
}

func TestImportPreflightMissingTeam(t *testing.T) {
	s, f, _ := newService(t, true)
	f.fail("GET", "orgs/"+org+"/teams/graders", errNotFound)
	_, err := s.ImportStudents([]canvas.Student{stu(2, "A", "a")}, importMeta(), nil)
	if err == nil || !strings.Contains(err.Error(), "team 'graders' does not exist") {
		t.Fatalf("err = %v", err)
	}
}

func TestImportPreflightMissingSchema(t *testing.T) {
	// Apply: stop before touching anything.
	s, f, _ := newService(t, true)
	f.team().on("GET", "orgs/"+org+"/properties/schema", `[{"property_name":"repo_type"}]`)
	_, err := s.ImportStudents([]canvas.Student{stu(2, "A", "a")}, importMeta(), nil)
	if err == nil || !strings.Contains(err.Error(), "student_name, github_id") || !strings.Contains(err.Error(), "properties setup") {
		t.Fatalf("err = %v", err)
	}

	// Dry run: warn and keep going so the whole plan is visible.
	s, f, out := newService(t, false)
	f.team().on("GET", "orgs/"+org+"/properties/schema", `[]`).repos(`[]`).user("a")
	if _, err := s.ImportStudents([]canvas.Student{stu(2, "A", "a")}, importMeta(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "WARNING   custom properties not defined") || !strings.Contains(out.String(), "WOULD CREATE") {
		t.Errorf("output:\n%s", out)
	}
}

func TestImportReportsNormalizedID(t *testing.T) {
	s, f, out := newService(t, false)
	importFixture(f, `[]`).user("jsmith")
	if _, err := s.ImportStudents([]canvas.Student{stu(2, "Jane", "https://github.com/jsmith")}, importMeta(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `(GitHub-ID entered as "https://github.com/jsmith")`) {
		t.Errorf("output:\n%s", out)
	}
}

func TestConcise(t *testing.T) {
	err := errWithArgs("gh: Not Found (HTTP 404)")
	if got := concise(err); got != "gh: Not Found (HTTP 404)" {
		t.Errorf("concise = %q", got)
	}
	if got := concise(errors.New("a\n  b")); got != "a b" {
		t.Errorf("concise = %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Error(got)
	}
	if got := truncate("abcdefghij", 5); got != "abcd~" {
		t.Error(got)
	}
}
