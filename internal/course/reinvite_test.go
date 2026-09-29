package course

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/roster"
)

// expiredRepoInvite: invited 10 days before pinNow.
func studentWithInvite(f *fakeGH, repo, login, invitations string) *fakeGH {
	return f.team().user(login).
		on("GET", repoPath(repo, ""), `{"name":"`+repo+`","html_url":"u/`+repo+`"}`).
		on("GET", repoPath(repo, "properties/values"), `[{"property_name":"repo_type","value":"student"},{"property_name":"student_name","value":"Dave"},{"property_name":"github_id","value":"`+login+`"}]`).
		fail("GET", repoPath(repo, "collaborators/"+login), errNotFound).
		on("GET", repoPath(repo, "invitations"), invitations).
		on("GET", teamRepoPath(repo), `{"role_name":"write"}`).
		on("GET", repoPath(repo, "commits?per_page=2"), `[{},{}]`)
}

const expiredInvite = `[{"id":987654321,"invitee":{"login":"dave"},"created_at":"2026-09-19T12:00:00Z"}]`
const liveInvite = `[{"id":5,"invitee":{"login":"dave"},"created_at":"2026-09-28T12:00:00Z"}]`

func TestRepairReinvitesExpiredStudentInvite(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, true)
	studentWithInvite(f, "dave", "dave", expiredInvite).
		on("DELETE", repoPath("dave", "invitations/987654321"), ``).
		on("PUT", repoPath("dave", "collaborators/dave"), `{}`)

	res, err := s.CreateStudentRepo("Dave", "dave", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultRepaired {
		t.Fatalf("result = %q\n%s", res, out)
	}
	want := []string{"DELETE " + repoPath("dave", "invitations/987654321"), "PUT " + repoPath("dave", "collaborators/dave")}
	if got := f.mutations(); !reflect.DeepEqual(got, want) {
		t.Errorf("mutations = %v, want cancel then re-add %v", got, want)
	}
	if !strings.Contains(out.String(), "re-invite dave with: push (previous invitation expired)") {
		t.Errorf("output:\n%s", out)
	}
}

func TestRepairReinviteDryRun(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	studentWithInvite(f, "dave", "dave", expiredInvite)
	if _, err := s.CreateStudentRepo("Dave", "dave", ""); err != nil {
		t.Fatal(err)
	}
	if len(f.mutations()) != 0 || !strings.Contains(out.String(), "would repair") || !strings.Contains(out.String(), "previous invitation expired") {
		t.Errorf("mutations %v\n%s", f.mutations(), out)
	}
}

func TestRepairLeavesLiveInviteAlone(t *testing.T) {
	pinNow(t)
	s, f, _ := newService(t, true)
	studentWithInvite(f, "dave", "dave", liveInvite)
	res, err := s.CreateStudentRepo("Dave", "dave", "")
	if err != nil {
		t.Fatal(err)
	}
	if res != ResultExisting || len(f.mutations()) != 0 {
		t.Errorf("result %q mutations %v", res, f.mutations())
	}
}

func TestRepairHonorsAPIExpiredFlag(t *testing.T) {
	pinNow(t)
	s, f, _ := newService(t, false)
	studentWithInvite(f, "dave", "dave", `[{"id":6,"invitee":{"login":"dave"},"created_at":"2026-09-29T11:00:00Z","expired":true}]`)
	res, _ := s.CreateStudentRepo("Dave", "dave", "")
	if res != ResultDryRun {
		t.Errorf("expired flag from API should trigger a re-invite plan, got %q", res)
	}
}

func TestRepairReinviteToleratesAlreadyCancelledInvite(t *testing.T) {
	pinNow(t)
	s, f, _ := newService(t, true)
	studentWithInvite(f, "dave", "dave", expiredInvite).
		fail("DELETE", repoPath("dave", "invitations/987654321"), errNotFound).
		on("PUT", repoPath("dave", "collaborators/dave"), `{}`)
	if res, err := s.CreateStudentRepo("Dave", "dave", ""); err != nil || res != ResultRepaired {
		t.Fatalf("res %q err %v", res, err)
	}
}

func TestImportRepairReinvitesExpired(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, true)
	importFixture(f, `[{"name":"dave","html_url":"u/dave","custom_properties":{"repo_type":"student","student_name":"Dave","github_id":"dave"}}]`)
	studentWithInvite(f, "dave", "dave", expiredInvite).
		on("DELETE", repoPath("dave", "invitations/987654321"), ``).
		on("PUT", repoPath("dave", "collaborators/dave"), `{}`)
	sum, err := s.ImportStudents([]roster.Student{stu(2, "Dave", "dave")}, repairMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Repaired != 1 || !strings.Contains(out.String(), "REPAIRING    SUCCESS  dave: re-invite dave") {
		t.Errorf("sum %+v\n%s", sum, out)
	}
}

func TestImportWithoutRepairDoesNotReinvite(t *testing.T) {
	pinNow(t)
	s, f, _ := newService(t, true)
	importFixture(f, `[{"name":"dave","html_url":"u/dave","custom_properties":{"repo_type":"student","student_name":"Dave","github_id":"dave"}}]`)
	sum, _ := s.ImportStudents([]roster.Student{stu(2, "Dave", "dave")}, importMeta(), nil)
	if sum.Skipped != 1 || len(f.mutations()) != 0 {
		t.Errorf("plain import must not touch existing repos: %+v %v", sum, f.mutations())
	}
}

// ---------------------------------------------------------------- team

func TestTeamAddReinvitesExpired(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, true)
	teamRoutes(f, `[]`, teamInvitesJSON).user("ta-cy").
		on("DELETE", "orgs/"+org+"/invitations/2", ``).
		on("PUT", "orgs/"+org+"/teams/graders/memberships/ta-cy", `{"state":"pending"}`)
	if err := s.AddTeamMember("graders", "ta-cy"); err != nil {
		t.Fatal(err)
	}
	want := []string{"DELETE orgs/" + org + "/invitations/2", "PUT orgs/" + org + "/teams/graders/memberships/ta-cy"}
	if got := f.mutations(); !reflect.DeepEqual(got, want) {
		t.Errorf("mutations = %v, want %v", got, want)
	}
	if !strings.Contains(out.String(), "REINVITED ta-cy") || !strings.Contains(out.String(), orgInviteURL) {
		t.Errorf("output:\n%s", out)
	}
}

func TestTeamAddReinviteDryRun(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	teamRoutes(f, `[]`, teamInvitesJSON).user("ta-cy")
	if err := s.AddTeamMember("graders", "ta-cy"); err != nil {
		t.Fatal(err)
	}
	if len(f.mutations()) != 0 || !strings.Contains(out.String(), "would re-invite ta-cy to 'graders' (previous invitation expired)") {
		t.Errorf("mutations %v\n%s", f.mutations(), out)
	}
}

func TestTeamAddReinviteFailedInvite(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, true)
	teamRoutes(f, `[]`, `[{"id":7,"login":"ta-fx","created_at":"2026-09-28T12:00:00Z","failed_at":"2026-09-28T13:00:00Z","failed_reason":"Invitation expired"}]`).user("ta-fx").
		fail("DELETE", "orgs/"+org+"/invitations/7", errNotFound). // already gone: fine
		on("PUT", "orgs/"+org+"/teams/graders/memberships/ta-fx", `{"state":"pending"}`)
	if err := s.AddTeamMember("graders", "ta-fx"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "REINVITED ta-fx") {
		t.Errorf("output:\n%s", out)
	}
}

func TestTeamInvitesShowsReinviteCommand(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	teamRoutes(f, `[]`, teamInvitesJSON)
	if _, err := s.TeamInvitesReport("graders", "ta-cy"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Re-invite:    mgc --apply team add ta-cy") {
		t.Errorf("output:\n%s", out)
	}
}
