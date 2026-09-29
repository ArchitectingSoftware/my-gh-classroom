package course

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
)

func TestStr(t *testing.T) {
	if got := str(nil); got != "" {
		t.Errorf("str(nil) = %q, want empty", got)
	}
	if got := str("x"); got != "x" {
		t.Errorf("str(x) = %q", got)
	}
	if got := str(true); got != "true" {
		t.Errorf("str(true) = %q", got)
	}
}

func TestStudentReadmeUsesCourseName(t *testing.T) {
	got := studentReadme("CS281", "Jane Smith", "jsmith42", "https://example.com/info")
	if strings.Contains(got, "CS472") {
		t.Fatalf("README still hardcodes CS472:\n%s", got)
	}
	for _, want := range []string{"# CS281 Student Repository", "repository for CS281", "Jane Smith", "jsmith42", "https://example.com/info"} {
		if !strings.Contains(got, want) {
			t.Errorf("README missing %q", want)
		}
	}
}

func TestCourseLabelFallsBackToOrganization(t *testing.T) {
	c := testClassroom()
	c.CourseName = ""
	if got := courseLabel(c); got != org {
		t.Errorf("courseLabel = %q, want %q", got, org)
	}
}

// ---------------------------------------------------------------- create

func TestCreateStudentRepoDryRunMakesNoMutations(t *testing.T) {
	s, f, out := newService(t, false)
	f.team().user("jsmith42").fail("GET", repoPath("jsmith42", ""), errNotFound)

	res, err := s.CreateStudentRepo("Jane Smith", "jsmith42", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultDryRun {
		t.Errorf("result = %q, want %q", res, ResultDryRun)
	}
	if m := f.mutations(); len(m) != 0 {
		t.Errorf("dry run performed mutations: %v", m)
	}
	if !strings.Contains(out.String(), "DRY RUN   would create private repo "+org+"/jsmith42") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestCreateStudentRepoApply(t *testing.T) {
	s, f, out := newService(t, true)
	f.team().user("JSmith42").
		fail("GET", repoPath("jsmith42", ""), errNotFound).
		on("POST", "orgs/"+org+"/repos", `{}`).
		on("GET", repoPath("jsmith42", "contents/README.md"), `{"sha":"abc123"}`).
		on("PUT", repoPath("jsmith42", "contents/README.md"), `{}`).
		on("PUT", teamRepoPath("jsmith42"), ``).
		on("PUT", repoPath("jsmith42", "collaborators/JSmith42"), `{}`).
		on("PATCH", repoPath("jsmith42", "properties/values"), ``)

	res, err := s.CreateStudentRepo("Jane Smith", "JSmith42", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultCreated {
		t.Errorf("result = %q", res)
	}

	want := []string{
		"POST orgs/" + org + "/repos",
		"PUT " + repoPath("jsmith42", "contents/README.md"),
		"PUT " + teamRepoPath("jsmith42"),
		"PUT " + repoPath("jsmith42", "collaborators/JSmith42"),
		"PATCH " + repoPath("jsmith42", "properties/values"),
	}
	if got := f.mutations(); !reflect.DeepEqual(got, want) {
		t.Errorf("mutations:\n got %v\nwant %v", got, want)
	}

	create, _ := f.find("POST", "orgs/"+org+"/repos")
	if !contains(create.Fields, "description=CS281 student repository for Jane Smith") {
		t.Errorf("repo description not course-specific: %v", create.Fields)
	}
	if !contains(create.Fields, "name=jsmith42") {
		t.Errorf("repo name should be lowercase: %v", create.Fields)
	}
	if !contains(create.Fields, "private=true") {
		t.Errorf("repo not private: %v", create.Fields)
	}

	readme, _ := f.find("PUT", repoPath("jsmith42", "contents/README.md"))
	if !contains(readme.Fields, "sha=abc123") {
		t.Errorf("README update missing sha: %v", readme.Fields)
	}
	content := decodeField(t, readme.Fields, "content=")
	if !strings.Contains(content, "# CS281 Student Repository") {
		t.Errorf("README content wrong:\n%s", content)
	}

	props, _ := f.find("PATCH", repoPath("jsmith42", "properties/values"))
	if !contains(props.Args, "--input") {
		t.Errorf("properties PATCH must pass --input - so gh reads the body: %v", props.Args)
	}
	got := decodeProps(t, props.Input)
	wantProps := map[string]string{"repo_type": "student", "student_name": "Jane Smith", "github_id": "JSmith42"}
	if !reflect.DeepEqual(got, wantProps) {
		t.Errorf("properties = %v, want %v", got, wantProps)
	}
	if !strings.Contains(out.String(), "CREATED   Jane Smith (JSmith42)") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestCreateStudentRepoCustomRepoName(t *testing.T) {
	s, f, _ := newService(t, false)
	f.team().user("jsmith42").fail("GET", repoPath("jane-smith", ""), errNotFound)
	if _, err := s.CreateStudentRepo("Jane", "jsmith42", "jane-smith"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateStudentRepoRejectsInvalidRepoName(t *testing.T) {
	s, f, _ := newService(t, false)
	f.team().user("jsmith42")
	if _, err := s.CreateStudentRepo("Jane", "jsmith42", "bad name!"); err == nil {
		t.Fatal("expected error for invalid repo name")
	}
}

func TestCreateStudentRepoRequiresGraderTeam(t *testing.T) {
	s, f, _ := newService(t, true)
	f.fail("GET", "orgs/"+org+"/teams/graders", errNotFound)
	_, err := s.CreateStudentRepo("Jane", "jsmith42", "")
	if err == nil || !strings.Contains(err.Error(), "team 'graders' does not exist") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateStudentRepoUnknownUser(t *testing.T) {
	s, f, _ := newService(t, true)
	f.team().fail("GET", "users/nobody", errNotFound)
	_, err := s.CreateStudentRepo("Jane", "nobody", "")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v", err)
	}
	if m := f.mutations(); len(m) != 0 {
		t.Errorf("mutations after failed user lookup: %v", m)
	}
}

func TestCreateStudentRepoPartialFailureSaysRerun(t *testing.T) {
	s, f, _ := newService(t, true)
	f.team().user("jsmith42").
		fail("GET", repoPath("jsmith42", ""), errNotFound).
		on("POST", "orgs/"+org+"/repos", `{}`).
		on("GET", repoPath("jsmith42", "contents/README.md"), `{"sha":"abc"}`).
		on("PUT", repoPath("jsmith42", "contents/README.md"), `{}`).
		on("PUT", teamRepoPath("jsmith42"), ``).
		fail("PUT", repoPath("jsmith42", "collaborators/jsmith42"), errors.New("boom"))

	_, err := s.CreateStudentRepo("Jane", "jsmith42", "")
	if err == nil || !strings.Contains(err.Error(), "repair it with: mgc --apply student create --name \"Jane\" --github jsmith42 --repo jsmith42") {
		t.Fatalf("err = %v, want re-run hint", err)
	}
}

// ---------------------------------------------------------------- reconcile existing repos

// existingRepo registers a repository that already has history beyond
// GitHub's initial commit, so the README is never a repair candidate.
func existingRepo(f *fakeGH, repo string) *fakeGH {
	return f.on("GET", repoPath(repo, ""), `{"name":"`+repo+`","html_url":"https://github.com/`+org+`/`+repo+`"}`).
		on("GET", repoPath(repo, "commits?per_page=2"), `[{"commit":{"message":"Initialize course README"}},{"commit":{"message":"Initial commit"}}]`)
}

func TestExistingFullyProvisionedRepoIsSkipped(t *testing.T) {
	s, f, out := newService(t, true)
	existingRepo(f.team().user("jsmith42"), "jsmith42").
		on("GET", repoPath("jsmith42", "properties/values"), `[{"property_name":"repo_type","value":"student"},{"property_name":"student_name","value":"Jane"},{"property_name":"github_id","value":"jsmith42"}]`).
		on("GET", repoPath("jsmith42", "collaborators/jsmith42"), ``).
		on("GET", teamRepoPath("jsmith42"), `{"role_name":"write"}`)

	res, err := s.CreateStudentRepo("Jane", "jsmith42", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultExisting {
		t.Errorf("result = %q", res)
	}
	if m := f.mutations(); len(m) != 0 {
		t.Errorf("mutations on fully provisioned repo: %v", m)
	}
	if !strings.Contains(out.String(), "SKIP      jsmith42 already exists") {
		t.Errorf("output:\n%s", out)
	}
}

func partiallyProvisioned(f *fakeGH) *fakeGH {
	return existingRepo(f.team().user("jsmith42"), "jsmith42").
		on("GET", repoPath("jsmith42", "properties/values"), `[]`).
		fail("GET", repoPath("jsmith42", "collaborators/jsmith42"), errNotFound).
		on("GET", repoPath("jsmith42", "invitations"), `[]`).
		fail("GET", teamRepoPath("jsmith42"), errNotFound)
}

func TestExistingRepoRepairDryRun(t *testing.T) {
	s, f, out := newService(t, false)
	partiallyProvisioned(f)

	res, err := s.CreateStudentRepo("Jane", "jsmith42", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultDryRun {
		t.Errorf("result = %q", res)
	}
	if m := f.mutations(); len(m) != 0 {
		t.Errorf("dry run performed mutations: %v", m)
	}
	for _, want := range []string{"would repair", "add student collaborator jsmith42", "add team 'graders'", "github_id=jsmith42"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestExistingRepoRepairApply(t *testing.T) {
	s, f, _ := newService(t, true)
	partiallyProvisioned(f).
		on("PUT", repoPath("jsmith42", "collaborators/jsmith42"), `{}`).
		on("PUT", teamRepoPath("jsmith42"), ``).
		on("PATCH", repoPath("jsmith42", "properties/values"), ``)

	res, err := s.CreateStudentRepo("Jane", "jsmith42", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultRepaired {
		t.Errorf("result = %q", res)
	}
	for _, m := range f.mutations() {
		if strings.Contains(m, "contents/") || strings.HasPrefix(m, "POST") {
			t.Errorf("repair must not touch repository contents or recreate: %s", m)
		}
	}
	if len(f.mutations()) != 3 {
		t.Errorf("mutations = %v", f.mutations())
	}
}

func TestExistingRepoRepairOnlyFillsMissingProperties(t *testing.T) {
	s, f, _ := newService(t, true)
	existingRepo(f.team().user("jsmith42"), "jsmith42").
		on("GET", repoPath("jsmith42", "properties/values"), `[{"property_name":"repo_type","value":"student"},{"property_name":"student_name","value":"Janet Smith"}]`).
		on("GET", repoPath("jsmith42", "collaborators/jsmith42"), ``).
		on("GET", teamRepoPath("jsmith42"), `{"role_name":"write"}`).
		on("PATCH", repoPath("jsmith42", "properties/values"), ``)

	if _, err := s.CreateStudentRepo("Jane", "jsmith42", ""); err != nil {
		t.Fatal(err)
	}
	c, _ := f.find("PATCH", repoPath("jsmith42", "properties/values"))
	got := decodeProps(t, c.Input)
	if !reflect.DeepEqual(got, map[string]string{"github_id": "jsmith42"}) {
		t.Errorf("should only set missing github_id, got %v", got)
	}
}

func TestExistingRepoPendingInvitationCountsAsAccess(t *testing.T) {
	s, f, _ := newService(t, true)
	existingRepo(f.team().user("jsmith42"), "jsmith42").
		on("GET", repoPath("jsmith42", "properties/values"), `[{"property_name":"repo_type","value":"student"},{"property_name":"student_name","value":"Jane"},{"property_name":"github_id","value":"jsmith42"}]`).
		fail("GET", repoPath("jsmith42", "collaborators/jsmith42"), errNotFound).
		on("GET", repoPath("jsmith42", "invitations"), `[{"invitee":{"login":"JSmith42"}}]`).
		on("GET", teamRepoPath("jsmith42"), `{"role_name":"write"}`)

	res, err := s.CreateStudentRepo("Jane", "jsmith42", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultExisting {
		t.Errorf("result = %q; pending invite should not trigger a re-invite", res)
	}
}

func TestExistingRepoOwnedBySomeoneElse(t *testing.T) {
	s, f, _ := newService(t, true)
	existingRepo(f.team().user("jsmith42"), "shared").
		on("GET", repoPath("shared", "properties/values"), `[{"property_name":"repo_type","value":"student"},{"property_name":"github_id","value":"other"}]`)
	_, err := s.CreateStudentRepo("Jane", "jsmith42", "shared")
	if err == nil || !strings.Contains(err.Error(), "belongs to GitHub user 'other'") {
		t.Fatalf("err = %v", err)
	}
	if m := f.mutations(); len(m) != 0 {
		t.Errorf("mutations: %v", m)
	}
}

func TestExistingNonStudentRepoRefused(t *testing.T) {
	s, f, _ := newService(t, true)
	existingRepo(f.team().user("jsmith42"), "course-info").
		on("GET", repoPath("course-info", "properties/values"), `[{"property_name":"repo_type","value":"course"}]`)
	_, err := s.CreateStudentRepo("Jane", "jsmith42", "course-info")
	if err == nil || !strings.Contains(err.Error(), "not a student repository") {
		t.Fatalf("err = %v", err)
	}
}

// ---------------------------------------------------------------- inspection

const repoList = `[
 {"name":"course-info","html_url":"u/course-info","custom_properties":{}},
 {"name":"scratch","html_url":"u/scratch"},
 {"name":"jsmith42","html_url":"u/jsmith42","custom_properties":{"repo_type":"student","student_name":"Jane Smith","github_id":"jsmith42"}},
 {"name":"janes-repo","html_url":"u/janes-repo","custom_properties":{"repo_type":"student","student_name":"Janet Jones","github_id":"jjones"}},
 {"name":"bob2","html_url":"u/bob2","custom_properties":{"repo_type":"student","student_name":null,"github_id":"bob2"}}
]`

func TestStudentListFiltersToStudentRepos(t *testing.T) {
	s, f, out := newService(t, false)
	f.repos(repoList)
	if err := s.StudentList(); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "("+"3):") {
		t.Errorf("expected 3 students:\n%s", o)
	}
	if strings.Contains(o, "scratch") || strings.Contains(o, "u/course-info") {
		t.Errorf("non-student repos listed:\n%s", o)
	}
	if strings.Contains(o, "<nil>") {
		t.Errorf("output contains <nil>:\n%s", o)
	}
	if !strings.Contains(o, "2 other repositories") {
		t.Errorf("expected note about other repos:\n%s", o)
	}
}

func TestStudentFindCountsMatches(t *testing.T) {
	s, f, out := newService(t, false)
	f.repos(repoList)
	if err := s.StudentFind("jan"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "Matches (2):") {
		t.Errorf("want 2 matches:\n%s", o)
	}
	if strings.Contains(o, "<nil>") {
		t.Errorf("output contains <nil>:\n%s", o)
	}
}

func TestStudentFindNoMatch(t *testing.T) {
	s, f, out := newService(t, false)
	f.repos(repoList)
	if err := s.StudentFind("scratch"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No matching student repositories found.") {
		t.Errorf("output:\n%s", out)
	}
}

func TestStudentFindEmptyQuery(t *testing.T) {
	s, _, _ := newService(t, false)
	if err := s.StudentFind("  "); err == nil {
		t.Fatal("expected error")
	}
}

func TestStudentInfoFindsCustomRepoNameAndSurvivesOddInvites(t *testing.T) {
	s, f, out := newService(t, false)
	f.repos(repoList).user("jjones").team().
		on("GET", repoPath("janes-repo", "collaborators/jjones/permission"), `{"permission":"write"}`).
		on("GET", teamRepoPath("janes-repo"), `{"permissions":{"pull":true,"push":true}}`).
		on("GET", repoPath("janes-repo", "invitations"), `[{"invitee":null},{"invitee":{"login":"jjones"}}]`)

	if err := s.StudentInfo("JJones"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	for _, want := range []string{"Student:      Janet Jones", "Repository:   u/janes-repo", "Student perm: push", "Grader team:  graders (push)", "Pending inv.: 2"} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
}

func TestStudentInfoNotCreated(t *testing.T) {
	s, f, out := newService(t, false)
	f.repos(repoList).user("newkid")
	if err := s.StudentInfo("newkid"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "NOT CREATED") {
		t.Errorf("output:\n%s", out)
	}
}

func TestTeamRepoPermission(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"role_name":"write"}`, "push"},
		{`{"role_name":"maintain"}`, "maintain"},
		{`{"permissions":{"admin":false,"push":true,"pull":true}}`, "push"},
		{`{}`, "connected"},
	}
	for _, c := range cases {
		s, f, _ := newService(t, false)
		f.on("GET", teamRepoPath("r"), c.body)
		got, ok, err := s.TeamRepoPermission("graders", "r")
		if err != nil || !ok || got != c.want {
			t.Errorf("%s: got (%q, %v, %v), want %q", c.body, got, ok, err, c.want)
		}
	}
	s, f, _ := newService(t, false)
	f.fail("GET", teamRepoPath("r"), errNotFound)
	if _, ok, err := s.TeamRepoPermission("graders", "r"); ok || err != nil {
		t.Errorf("404 should be (not connected, nil), got (%v, %v)", ok, err)
	}
}

// ---------------------------------------------------------------- teams

func TestRemoveTeamMemberIsCaseInsensitive(t *testing.T) {
	s, f, out := newService(t, true)
	f.team().
		on("GET", "orgs/"+org+"/teams/graders/members", `[{"login":"TA-Alice"}]`).
		on("DELETE", "orgs/"+org+"/teams/graders/memberships/TA-Alice", ``)
	if err := s.RemoveTeamMember("graders", "ta-alice"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "REMOVED   TA-Alice") {
		t.Errorf("output:\n%s", out)
	}
}

func TestRemoveTeamMemberNotMember(t *testing.T) {
	s, f, out := newService(t, true)
	f.team().on("GET", "orgs/"+org+"/teams/graders/members", `[{"login":"someone"}]`).
		on("GET", "orgs/"+org+"/teams/graders/invitations", `[]`)
	if err := s.RemoveTeamMember("graders", "ta-alice"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "SKIP") || len(f.mutations()) != 0 {
		t.Errorf("expected skip without mutation:\n%s", out)
	}
}

func TestAddTeamMemberSkipsExisting(t *testing.T) {
	s, f, out := newService(t, true)
	f.team().
		on("GET", "users/ta-alice", `{"login":"TA-Alice"}`). // GitHub returns the canonical case
		on("GET", "orgs/"+org+"/teams/graders/members", `[{"login":"TA-Alice"}]`)
	if err := s.AddTeamMember("graders", "ta-alice"); err != nil {
		t.Fatal(err)
	}
	if len(f.mutations()) != 0 || !strings.Contains(out.String(), "SKIP") {
		t.Errorf("expected skip:\n%s", out)
	}
}

func TestCreateTeamDryRun(t *testing.T) {
	s, f, out := newService(t, false)
	f.fail("GET", "orgs/"+org+"/teams/graders", errNotFound)
	if err := s.CreateTeam("graders"); err != nil {
		t.Fatal(err)
	}
	if len(f.mutations()) != 0 || !strings.Contains(out.String(), "DRY RUN   would create team") {
		t.Errorf("output:\n%s", out)
	}
}

// ---------------------------------------------------------------- properties

func TestPropertiesSetupIsOrderedAndSkipsExisting(t *testing.T) {
	s, f, out := newService(t, true)
	f.on("GET", "orgs/"+org+"/properties/schema", `[{"property_name":"student_name"}]`).
		on("PUT", "orgs/"+org+"/properties/schema/repo_type", `{}`).
		on("PUT", "orgs/"+org+"/properties/schema/github_id", `{}`)
	if err := s.PropertiesSetup(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"PUT orgs/" + org + "/properties/schema/repo_type",
		"PUT orgs/" + org + "/properties/schema/github_id",
	}
	if got := f.mutations(); !reflect.DeepEqual(got, want) {
		t.Errorf("mutations = %v, want %v", got, want)
	}
	for _, c := range f.calls {
		if c.Method == "PUT" && !contains(c.Args, "--input") {
			t.Errorf("schema PUT must pass --input -: %v", c.Args)
		}
	}
	if !strings.Contains(out.String(), "SKIP      custom property 'student_name'") {
		t.Errorf("output:\n%s", out)
	}
}

// ---------------------------------------------------------------- helpers

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func decodeField(t *testing.T, fields []string, prefix string) string {
	t.Helper()
	for _, f := range fields {
		if strings.HasPrefix(f, prefix) {
			b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(f, prefix))
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	t.Fatalf("field %s not found in %v", prefix, fields)
	return ""
}

func decodeProps(t *testing.T, input []byte) map[string]string {
	t.Helper()
	var body struct {
		Properties []struct {
			Name  string `json:"property_name"`
			Value string `json:"value"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(input, &body); err != nil {
		t.Fatalf("bad properties body %q: %v", input, err)
	}
	out := map[string]string{}
	for _, p := range body.Properties {
		out[p.Name] = p.Value
	}
	return out
}

func TestRepoNameCase(t *testing.T) {
	cases := []struct {
		name     string
		caseMode string
		keepCase bool
		want     string
	}{
		{"default lowercases", "", false, "cs281-jsmith42"},
		{"lower lowercases", config.RepoNameCaseLower, false, "cs281-jsmith42"},
		{"preserve keeps case", config.RepoNameCasePreserve, false, "CS281-JSmith42"},
		{"--keep-case overrides lower", config.RepoNameCaseLower, true, "CS281-JSmith42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := newService(t, false)
			s.C.RepoPrefix = "CS281"
			s.C.RepoNameCase = tc.caseMode
			s.KeepCase = tc.keepCase
			if got := s.RepoName("JSmith42"); got != tc.want {
				t.Errorf("RepoName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCreateStudentRepoKeepCaseDryRun(t *testing.T) {
	s, f, out := newService(t, false)
	s.KeepCase = true
	f.team().on("GET", "users/jsmith42", `{"login":"JSmith42"}`).fail("GET", repoPath("JSmith42", ""), errNotFound)
	if _, err := s.CreateStudentRepo("Jane Smith", "jsmith42", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would create private repo "+org+"/JSmith42") {
		t.Errorf("expected case-preserved repo name (login as GitHub reports it):\n%s", out)
	}
}
