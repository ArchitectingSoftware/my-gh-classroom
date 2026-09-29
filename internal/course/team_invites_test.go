package course

import (
	"strings"
	"testing"
)

const orgInviteURL = "https://github.com/orgs/" + org + "/invitation"

func teamRoutes(f *fakeGH, members, invites string) *fakeGH {
	return f.team().
		on("GET", "orgs/"+org+"/teams/graders/members", members).
		on("GET", "orgs/"+org+"/teams/graders/invitations", invites)
}

// Relative to pinNow (2026-09-29 12:00 UTC): ta-bo invited 1 day ago,
// ta-cy 9 days ago (expired), and one email-only invitation that failed.
const teamInvitesJSON = `[
 {"id":123456789,"login":"ta-bo","created_at":"2026-09-28T12:00:00Z"},
 {"id":2,"login":"ta-cy","created_at":"2026-09-20T12:00:00Z"},
 {"id":3,"login":null,"email":"dee@drexel.edu","created_at":"2026-09-28T12:00:00Z","failed_at":"2026-09-28T13:00:00Z","failed_reason":"Invitation expired"}
]`

func TestTeamListShowsPendingInvitations(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	teamRoutes(f, `[{"login":"ta-al"}]`, teamInvitesJSON)
	if err := s.TeamList("graders"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	for _, want := range []string{"(1 active, 3 invited)", "ta-al", "ta-bo", "invited, 6d left", "ta-cy", "invited, EXPIRED", "dee@drexel.edu", "invited, FAILED", orgInviteURL} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
}

func TestTeamInvitesReport(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	teamRoutes(f, `[]`, teamInvitesJSON)
	if err := s.TeamInvitesReport("graders", ""); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	for _, want := range []string{
		"Pending invitations to " + org + "/graders (3, 2 expired or failed)",
		"ta-bo", "pending, 6d left", orgInviteURL,
		"EXPIRED", "FAILED", "(invite again)",
	} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
	if strings.Count(o, orgInviteURL) != 1 {
		t.Errorf("only the usable invitation should get a URL:\n%s", o)
	}
	if len(f.mutations()) != 0 {
		t.Errorf("report changed something: %v", f.mutations())
	}
}

func TestTeamInvitesReportNone(t *testing.T) {
	s, f, out := newService(t, false)
	teamRoutes(f, `[]`, `[]`)
	if err := s.TeamInvitesReport("graders", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "everyone added to the team has accepted") {
		t.Errorf("output:\n%s", out)
	}
}

func TestTeamInviteOnePending(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	teamRoutes(f, `[]`, teamInvitesJSON)
	if err := s.TeamInvitesReport("graders", "TA-BO"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	for _, want := range []string{"Invitee:      ta-bo", "Invitation:   pending, 6d left", "Accept at:    " + orgInviteURL, "sign in to GitHub as ta-bo"} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
}

func TestTeamInviteOneFailedByEmail(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	teamRoutes(f, `[]`, teamInvitesJSON)
	if err := s.TeamInvitesReport("graders", "dee@drexel.edu"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "Invitation:   FAILED") || !strings.Contains(o, "Reason:       Invitation expired") || strings.Contains(o, "Accept at:    https") {
		t.Errorf("output:\n%s", o)
	}
}

func TestTeamInviteOneActiveOrUnknown(t *testing.T) {
	s, f, out := newService(t, false)
	teamRoutes(f, `[{"login":"TA-Al"}]`, `[]`)
	if err := s.TeamInvitesReport("graders", "ta-al"); err != nil {
		t.Fatal(err)
	}
	if err := s.TeamInvitesReport("graders", "ghost"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "TA-Al is already an active member") || !strings.Contains(o, "mgc --apply team add ghost") {
		t.Errorf("output:\n%s", o)
	}
}

func TestAddTeamMemberReportsInvitationURL(t *testing.T) {
	s, f, out := newService(t, true)
	teamRoutes(f, `[]`, `[]`).user("ta-new").
		on("PUT", "orgs/"+org+"/teams/graders/memberships/ta-new", `{"state":"pending","role":"member"}`)
	if err := s.AddTeamMember("graders", "ta-new"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "INVITED   ta-new") || !strings.Contains(out.String(), "accept at: "+orgInviteURL) {
		t.Errorf("output:\n%s", out)
	}
}

func TestAddTeamMemberAlreadyInOrg(t *testing.T) {
	s, f, out := newService(t, true)
	teamRoutes(f, `[]`, `[]`).user("ta-org").
		on("PUT", "orgs/"+org+"/teams/graders/memberships/ta-org", `{"state":"active","role":"member"}`)
	if err := s.AddTeamMember("graders", "ta-org"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ADDED     ta-org") {
		t.Errorf("output:\n%s", out)
	}
}

func TestAddTeamMemberSkipsPendingInvite(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, true)
	teamRoutes(f, `[]`, teamInvitesJSON).user("ta-bo")
	if err := s.AddTeamMember("graders", "ta-bo"); err != nil {
		t.Fatal(err)
	}
	if len(f.mutations()) != 0 || !strings.Contains(out.String(), "has already been invited") || !strings.Contains(out.String(), orgInviteURL) {
		t.Errorf("mutations %v\n%s", f.mutations(), out)
	}
}

func TestRemoveTeamMemberCancelsPendingInvite(t *testing.T) {
	pinNow(t)
	// dry run
	s, f, out := newService(t, false)
	teamRoutes(f, `[]`, teamInvitesJSON)
	if err := s.RemoveTeamMember("graders", "ta-bo"); err != nil {
		t.Fatal(err)
	}
	if len(f.mutations()) != 0 || !strings.Contains(out.String(), "would cancel the pending invitation for ta-bo") {
		t.Errorf("dry run: mutations %v\n%s", f.mutations(), out)
	}
	// apply: the large id must not be rendered in float notation
	s, f, out = newService(t, true)
	teamRoutes(f, `[]`, teamInvitesJSON).on("DELETE", "orgs/"+org+"/invitations/123456789", ``)
	if err := s.RemoveTeamMember("graders", "ta-bo"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CANCELLED invitation for ta-bo") {
		t.Errorf("output:\n%s", out)
	}
}

func TestRemoveActiveMemberStillWorks(t *testing.T) {
	s, f, out := newService(t, true)
	teamRoutes(f, `[{"login":"ta-al"}]`, `[]`).on("DELETE", "orgs/"+org+"/teams/graders/memberships/ta-al", ``)
	if err := s.RemoveTeamMember("graders", "ta-al"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "REMOVED   ta-al") {
		t.Errorf("output:\n%s", out)
	}
}
