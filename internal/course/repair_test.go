package course

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/roster"
)

// completeRepo registers an existing repository that needs nothing except,
// optionally, README work controlled by commits/readme.
func completeRepo(f *fakeGH, repo, login, commits string) *fakeGH {
	return f.
		on("GET", repoPath(repo, ""), `{"name":"`+repo+`","html_url":"u/`+repo+`"}`).
		on("GET", repoPath(repo, "properties/values"), `[{"property_name":"repo_type","value":"student"},{"property_name":"student_name","value":"Jane"},{"property_name":"github_id","value":"`+login+`"}]`).
		on("GET", repoPath(repo, "collaborators/"+login), ``).
		on("GET", teamRepoPath(repo), `{"role_name":"write"}`).
		on("GET", repoPath(repo, "commits?per_page=2"), commits)
}

func readmeJSON(content string) string {
	return `{"sha":"old-sha","content":"` + base64.StdEncoding.EncodeToString([]byte(content)) + `"}`
}

const onlyInitialCommit = `[{"commit":{"message":"Initial commit"}}]`

func TestRepairReplacesPlaceholderReadme(t *testing.T) {
	s, f, out := newService(t, true)
	completeRepo(f.team().user("jsmith42"), "jsmith42", "jsmith42", onlyInitialCommit).
		on("GET", repoPath("jsmith42", "contents/README.md"), readmeJSON("# jsmith42\nCS281 student repository for Jane\n")).
		on("PUT", repoPath("jsmith42", "contents/README.md"), `{}`)

	res, err := s.CreateStudentRepo("Jane", "jsmith42", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultRepaired {
		t.Fatalf("result = %q\n%s", res, out)
	}
	c, ok := f.find("PUT", repoPath("jsmith42", "contents/README.md"))
	if !ok || !contains(c.Fields, "sha=old-sha") {
		t.Fatalf("README not replaced with sha: %+v", c)
	}
	if got := decodeField(t, c.Fields, "content="); !strings.Contains(got, "# CS281 Student Repository") {
		t.Errorf("README content:\n%s", got)
	}
	if !strings.Contains(out.String(), "replace GitHub placeholder README") {
		t.Errorf("output:\n%s", out)
	}
}

func TestRepairLeavesReadmeAloneWhenStudentHasWorked(t *testing.T) {
	cases := map[string]struct{ commits, readme string }{
		"more than one commit":       {`[{"commit":{"message":"lab 1"}},{"commit":{"message":"Initial commit"}}]`, ""},
		"student's own first commit": {`[{"commit":{"message":"my first commit"}}]`, ""},
		"README edited in place":     {onlyInitialCommit, "# My notes\nstuff\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s, f, _ := newService(t, true)
			completeRepo(f.team().user("jsmith42"), "jsmith42", "jsmith42", c.commits)
			if c.readme != "" {
				f.on("GET", repoPath("jsmith42", "contents/README.md"), readmeJSON(c.readme))
			}
			res, err := s.CreateStudentRepo("Jane", "jsmith42", "")
			if err != nil {
				t.Fatal(err)
			}
			if res != ResultExisting || len(f.mutations()) != 0 {
				t.Errorf("result %q, mutations %v", res, f.mutations())
			}
		})
	}
}

func TestRepairEmptyRepoCreatesReadme(t *testing.T) {
	s, f, _ := newService(t, true)
	completeRepo(f.team().user("jsmith42"), "jsmith42", "jsmith42", "").
		fail("GET", repoPath("jsmith42", "commits?per_page=2"), &ghErr409).
		on("PUT", repoPath("jsmith42", "contents/README.md"), `{}`)
	if _, err := s.CreateStudentRepo("Jane", "jsmith42", ""); err != nil {
		t.Fatal(err)
	}
	c, ok := f.find("PUT", repoPath("jsmith42", "contents/README.md"))
	if !ok {
		t.Fatal("README not created")
	}
	for _, fld := range c.Fields {
		if strings.HasPrefix(fld, "sha=") {
			t.Errorf("creating a README must not send a sha: %v", c.Fields)
		}
	}
}

// ---------------------------------------------------------------- repo prefix

func prefixed(t *testing.T, apply bool) (*Service, *fakeGH, *strings.Builder) {
	s, f, _ := newService(t, apply)
	s.C.RepoPrefix = "cs281-"
	var out strings.Builder
	s.Out = &out
	return s, f, &out
}

func TestStudentCreateUsesRepoPrefix(t *testing.T) {
	s, f, out := prefixed(t, false)
	f.team().user("jsmith42").fail("GET", repoPath("cs281-jsmith42", ""), errNotFound)
	if _, err := s.CreateStudentRepo("Jane", "jsmith42", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would create private repo "+org+"/cs281-jsmith42") {
		t.Errorf("output:\n%s", out)
	}
}

func TestImportUsesRepoPrefix(t *testing.T) {
	s, f, out := prefixed(t, true)
	newRepoRoutes(importFixture(f, `[]`).user("alice1"), "cs281-alice1")
	// newRepoRoutes keys the collaborator route by repo; fix it up for the login.
	f.on("PUT", repoPath("cs281-alice1", "collaborators/alice1"), `{}`)

	sum, err := s.ImportStudents([]roster.Student{stu(2, "Alice", "alice1")}, importMeta(), nil)
	if err != nil || sum.Created != 1 {
		t.Fatalf("sum %+v err %v\n%s", sum, err, out)
	}
	create, _ := f.find("POST", "orgs/"+org+"/repos")
	if !contains(create.Fields, "name=cs281-alice1") {
		t.Errorf("repo not prefixed: %v", create.Fields)
	}
	if !strings.Contains(out.String(), "Repo names:   cs281-<github-id>") {
		t.Errorf("header should show prefix:\n%s", out)
	}
}

func TestImportFindsExistingRepoWithOrWithoutPrefix(t *testing.T) {
	s, f, out := prefixed(t, false)
	importFixture(f, `[
	 {"name":"cs281-alice1","html_url":"u/cs281-alice1"},
	 {"name":"bob2","html_url":"u/bob2"},
	 {"name":"old-name","html_url":"u/old-name","custom_properties":{"repo_type":"student","github_id":"carol3"}}
	]`)
	sum, err := s.ImportStudents([]roster.Student{
		stu(2, "Alice", "alice1"), // prefixed name
		stu(3, "Bob", "bob2"),     // created before a prefix was configured
		stu(4, "Carol", "carol3"), // found by github_id property
	}, importMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Skipped != 3 || len(f.mutations()) != 0 {
		t.Errorf("sum %+v mutations %v\n%s", sum, f.mutations(), out)
	}
}

func TestStudentInfoFindsPrefixedRepo(t *testing.T) {
	s, f, out := prefixed(t, false)
	f.repos(`[{"name":"cs281-jsmith42","html_url":"u/cs281-jsmith42"}]`).user("jsmith42").team().
		on("GET", repoPath("cs281-jsmith42", "collaborators/jsmith42/permission"), `{"permission":"write"}`).
		on("GET", teamRepoPath("cs281-jsmith42"), `{"role_name":"write"}`).
		on("GET", repoPath("cs281-jsmith42", "invitations"), `[]`)
	if err := s.StudentInfo("jsmith42"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Repository:   u/cs281-jsmith42") {
		t.Errorf("output:\n%s", out)
	}
}

// ---------------------------------------------------------------- import --repair

func repairMeta() ImportMeta { m := importMeta(); m.Repair = true; return m }

func brokenRepoRoutes(f *fakeGH, repo, login string) *fakeGH {
	return f.
		on("GET", repoPath(repo, "properties/values"), `[]`).
		fail("GET", repoPath(repo, "collaborators/"+login), errNotFound).
		on("GET", repoPath(repo, "invitations"), `[]`).
		on("GET", teamRepoPath(repo), `{"role_name":"write"}`).
		on("GET", repoPath(repo, "commits?per_page=2"), `[{},{}]`)
}

func TestImportWithoutRepairLeavesBrokenRepoAlone(t *testing.T) {
	s, f, out := newService(t, true)
	importFixture(f, `[{"name":"dave","html_url":"u/dave"}]`)
	sum, err := s.ImportStudents([]roster.Student{stu(2, "Dave", "dave")}, importMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Skipped != 1 || len(f.mutations()) != 0 || !strings.Contains(out.String(), "re-run with --repair") {
		t.Errorf("sum %+v mutations %v\n%s", sum, f.mutations(), out)
	}
}

func TestImportRepairDryRun(t *testing.T) {
	s, f, out := newService(t, false)
	brokenRepoRoutes(importFixture(f, `[{"name":"dave","html_url":"u/dave"}]`), "dave", "dave")
	sum, err := s.ImportStudents([]roster.Student{stu(2, "Dave Diaz", "dave")}, repairMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Repaired != 1 || len(f.mutations()) != 0 {
		t.Errorf("sum %+v mutations %v", sum, f.mutations())
	}
	o := out.String()
	for _, want := range []string{"WOULD REPAIR", "dave: would add student collaborator dave", "github_id=dave", "Would repair: 1", "Existing:     repair anything missing"} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
}

func TestImportRepairApply(t *testing.T) {
	s, f, out := newService(t, true)
	brokenRepoRoutes(importFixture(f, `[{"name":"dave","html_url":"u/dave"}]`), "dave", "dave").
		on("PUT", repoPath("dave", "collaborators/dave"), `{}`).
		on("PATCH", repoPath("dave", "properties/values"), ``)
	completeRepo(f, "erin", "erin", `[{},{}]`)
	f.repos(`[{"name":"dave","html_url":"u/dave"},{"name":"erin","html_url":"u/erin","custom_properties":{"repo_type":"student","github_id":"erin"}}]`)

	sum, err := s.ImportStudents([]roster.Student{stu(2, "Dave Diaz", "dave"), stu(3, "Erin", "erin")}, repairMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sameCounts(sum, ImportSummary{Processed: 2, Repaired: 1, Skipped: 1}) {
		t.Errorf("summary %+v\n%s", sum, out)
	}
	o := out.String()
	for _, want := range []string{"REPAIRING", "SUCCESS  dave: add student collaborator dave", "nothing to repair: u/erin", "Repaired:     1"} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
	for _, m := range f.mutations() {
		if strings.Contains(m, "erin") || strings.Contains(m, "contents/") {
			t.Errorf("unexpected mutation %s", m)
		}
	}
}

func TestImportRepairFailureIsReported(t *testing.T) {
	s, f, out := newService(t, true)
	brokenRepoRoutes(importFixture(f, `[{"name":"dave","html_url":"u/dave"}]`), "dave", "dave").
		fail("PUT", repoPath("dave", "collaborators/dave"), errWithArgs("gh: Validation Failed (HTTP 422)"))
	sum, err := s.ImportStudents([]roster.Student{stu(2, "Dave Diaz", "dave")}, repairMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Errors != 1 {
		t.Errorf("summary %+v", sum)
	}
	line := ""
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(l, "[1/1]") {
			line = l
		}
	}
	if strings.Count(line, "REPAIRING") != 1 || !strings.Contains(line, "ERROR    dave: repair failed: add student collaborator dave") || !strings.Contains(line, "HTTP 422") {
		t.Errorf("line = %q", line)
	}
}

func TestImportRepairRefusesOtherStudentsRepo(t *testing.T) {
	s, f, out := newService(t, true)
	importFixture(f, `[{"name":"dave","html_url":"u/dave","custom_properties":{"repo_type":"student","github_id":"someone"}}]`)
	sum, _ := s.ImportStudents([]roster.Student{stu(2, "Dave", "dave")}, repairMeta(), nil)
	if sum.Errors != 1 || len(f.mutations()) != 0 || !strings.Contains(out.String(), "belongs to GitHub user 'someone'") {
		t.Errorf("sum %+v\n%s", sum, out)
	}
}

func TestRepoPrefixWithoutDashGetsOne(t *testing.T) {
	s, f, out := prefixed(t, false)
	s.C.RepoPrefix = "cs281" // no trailing dash
	f.team().user("jsmith42").fail("GET", repoPath("cs281-jsmith42", ""), errNotFound)
	if _, err := s.CreateStudentRepo("Jane", "jsmith42", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would create private repo "+org+"/cs281-jsmith42") {
		t.Errorf("output:\n%s", out)
	}
}

func TestImportFindsRepoWithUndashedPrefix(t *testing.T) {
	s, f, out := prefixed(t, false)
	s.C.RepoPrefix = "cs281"
	importFixture(f, `[{"name":"cs281-alice1","html_url":"u/cs281-alice1"}]`)
	sum, err := s.ImportStudents([]roster.Student{stu(2, "Alice", "alice1")}, importMeta(), nil)
	if err != nil || sum.Skipped != 1 || len(f.mutations()) != 0 {
		t.Fatalf("sum %+v err %v\n%s", sum, err, out)
	}
	if !strings.Contains(out.String(), "Repo names:   cs281-<github-id>") {
		t.Errorf("header should show the resolved prefix:\n%s", out)
	}
}

func TestImportLowercasesRepoNames(t *testing.T) {
	s, f, out := prefixed(t, true)
	s.C.RepoPrefix = "CS281"
	newRepoRoutes(importFixture(f, `[]`).user("SudoVoid1"), "cs281-sudovoid1")
	f.on("PUT", repoPath("cs281-sudovoid1", "collaborators/SudoVoid1"), `{}`)

	sum, err := s.ImportStudents([]roster.Student{stu(2, "Lucas", "SudoVoid1")}, importMeta(), nil)
	if err != nil || sum.Created != 1 {
		t.Fatalf("sum %+v err %v\n%s", sum, err, out)
	}
	create, _ := f.find("POST", "orgs/"+org+"/repos")
	if !contains(create.Fields, "name=cs281-sudovoid1") {
		t.Errorf("repo name not lowercased: %v", create.Fields)
	}
	props, _ := f.find("PATCH", repoPath("cs281-sudovoid1", "properties/values"))
	if got := decodeProps(t, props.Input)["github_id"]; got != "SudoVoid1" {
		t.Errorf("github_id should keep GitHub's case, got %q", got)
	}
}

func TestImportRecognizesEarlierMixedCaseRepo(t *testing.T) {
	s, f, _ := prefixed(t, true)
	s.C.RepoPrefix = "cs281"
	importFixture(f, `[{"name":"cs281-SudoVoid1","html_url":"u/cs281-SudoVoid1"}]`)
	sum, err := s.ImportStudents([]roster.Student{stu(2, "Lucas", "SudoVoid1")}, importMeta(), nil)
	if err != nil || sum.Skipped != 1 || len(f.mutations()) != 0 {
		t.Errorf("mixed-case repo from before should be skipped: sum %+v mutations %v", sum, f.mutations())
	}
}
