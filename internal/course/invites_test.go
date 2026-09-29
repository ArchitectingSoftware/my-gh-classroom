package course

import (
	"strings"
	"testing"
	"time"
)

// fixed "now" for invitation ages: 2026-09-29 12:00 UTC.
func pinNow(t *testing.T) {
	t.Helper()
	old := now
	now = func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = old })
}

const inviteRepos = `[
 {"name":"course-info","html_url":"u/course-info"},
 {"name":"notes","html_url":"u/notes","custom_properties":{"repo_type":"staff"}},
 {"name":"zed1","html_url":"u/zed1","custom_properties":{"repo_type":"student","student_name":"Zed Zulu","github_id":"zed1"}},
 {"name":"amy2","html_url":"u/amy2","custom_properties":{"repo_type":"student","student_name":"Amy Adams","github_id":"amy2"}},
 {"name":"bob3","html_url":"u/bob3","custom_properties":{"repo_type":"student","student_name":"Bob Baker","github_id":"bob3"}},
 {"name":"cs472-cat4","html_url":"u/cs472-cat4"}
]`

func inviteRoutes(f *fakeGH) *fakeGH {
	return f.repos(inviteRepos).
		// Zed: invited 2 days ago (5 days left)
		on("GET", repoPath("zed1", "invitations"), `[{"invitee":{"login":"zed1"},"created_at":"2026-09-27T12:00:00Z","html_url":"https://github.com/`+org+`/zed1/invitations"}]`).
		// Amy: invited 10 days ago (expired by age)
		on("GET", repoPath("amy2", "invitations"), `[{"invitee":{"login":"amy2"},"created_at":"2026-09-19T12:00:00Z","html_url":"https://github.com/`+org+`/amy2/invitations"}]`).
		// Bob: accepted, nothing pending
		on("GET", repoPath("bob3", "invitations"), `[]`).
		// Cat: untagged repo from an interrupted import; no html_url in response
		on("GET", repoPath("cs472-cat4", "invitations"), `[{"invitee":{"login":"cat4"},"created_at":"2026-09-29T06:00:00Z"}]`)
}

func TestInvitesReportForWholeOrg(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	inviteRoutes(f)
	if err := s.Invites(""); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if len(f.mutations()) != 0 {
		t.Errorf("report must not change anything: %v", f.mutations())
	}
	for _, want := range []string{
		"Pending repository invitations in " + org + " (3, 1 expired)",
		"https://github.com/" + org + "/zed1/invitations",
		"pending, 5d left",
		"EXPIRED",
		"https://github.com/" + org + "/cs472-cat4/invitations", // constructed when html_url is absent
		"pending, 6d left",
		"mgc --apply classroom import ROSTER.csv --repair",
		"(expired: invite again)",
	} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
	for _, unwanted := range []string{"Bob Baker", "course-info", "notes", "amy2/invitations"} {
		if strings.Contains(o, unwanted) {
			t.Errorf("output should not mention %q:\n%s", unwanted, o)
		}
	}
	// Sorted by student name: Amy, cat4 (login fallback), Zed.
	a, c, z := strings.Index(o, "Amy Adams"), strings.Index(o, "cat4"), strings.Index(o, "Zed Zulu")
	if !(a >= 0 && a < c && c < z) {
		t.Errorf("rows not sorted by name (amy=%d cat=%d zed=%d):\n%s", a, c, z, o)
	}
}

func TestInvitesReportNonePending(t *testing.T) {
	s, f, out := newService(t, false)
	f.repos(`[{"name":"bob3","custom_properties":{"repo_type":"student","github_id":"bob3"}}]`).
		on("GET", repoPath("bob3", "invitations"), `[]`)
	if err := s.Invites(""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "every invited student has accepted") {
		t.Errorf("output:\n%s", out)
	}
}

func TestInvitesReportContinuesPastUnreadableRepo(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	inviteRoutes(f).fail("GET", repoPath("amy2", "invitations"), errWithArgs("gh: Forbidden (HTTP 403)"))
	err := s.Invites("")
	if err == nil || !strings.Contains(err.Error(), "1 repositories") {
		t.Fatalf("err = %v", err)
	}
	o := out.String()
	if !strings.Contains(o, "Zed Zulu") || !strings.Contains(o, "amy2: gh: Forbidden (HTTP 403)") {
		t.Errorf("output:\n%s", o)
	}
}

func TestInviteForOneStudent(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	inviteRoutes(f)
	if err := s.Invites("ZED1"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	for _, want := range []string{
		"Student:      Zed Zulu",
		"Invitation:   pending, 5d left",
		"Accept at:    https://github.com/" + org + "/zed1/invitations",
		"sign in to GitHub as zed1",
		"Expires:",
	} {
		if !strings.Contains(o, want) {
			t.Errorf("output missing %q:\n%s", want, o)
		}
	}
}

func TestInviteForOneStudentByPrefixedRepo(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	s.C.RepoPrefix = "cs472-"
	inviteRoutes(f)
	if err := s.Invites("cat4"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Accept at:    https://github.com/"+org+"/cs472-cat4/invitations") {
		t.Errorf("output:\n%s", out)
	}
}

func TestInviteForOneStudentExpired(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	inviteRoutes(f)
	if err := s.Invites("amy2"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "Invitation:   EXPIRED") || strings.Contains(o, "Accept at:    https://") {
		t.Errorf("expired invitation must not offer a URL:\n%s", o)
	}
	if !strings.Contains(o, `Re-invite:    mgc --apply student create --name "Amy Adams" --github amy2 --repo amy2`) {
		t.Errorf("expired invitation should give the re-invite command:\n%s", o)
	}
}

func TestInviteExpiredFlagFromAPI(t *testing.T) {
	pinNow(t)
	s, f, out := newService(t, false)
	f.repos(`[{"name":"dan5","html_url":"u/dan5","custom_properties":{"repo_type":"student","github_id":"dan5"}}]`).
		on("GET", repoPath("dan5", "invitations"), `[{"invitee":{"login":"dan5"},"created_at":"2026-09-29T00:00:00Z","expired":true}]`)
	if err := s.Invites("dan5"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "EXPIRED") {
		t.Errorf("API expired flag should be honored:\n%s", out)
	}
}

func TestInviteForOneStudentAlreadyAccepted(t *testing.T) {
	s, f, out := newService(t, false)
	inviteRoutes(f).on("GET", repoPath("bob3", "collaborators/bob3"), ``)
	if err := s.Invites("bob3"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "already has access") {
		t.Errorf("output:\n%s", out)
	}
}

func TestInviteForOneStudentNoAccessNoInvite(t *testing.T) {
	s, f, out := newService(t, false)
	inviteRoutes(f).fail("GET", repoPath("bob3", "collaborators/bob3"), errNotFound)
	if err := s.Invites("bob3"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "does not have access") || !strings.Contains(out.String(), "--repair") {
		t.Errorf("output:\n%s", out)
	}
}

func TestInviteForUnknownStudent(t *testing.T) {
	s, f, _ := newService(t, false)
	inviteRoutes(f)
	if err := s.Invites("nobody"); err == nil || !strings.Contains(err.Error(), "no repository found for 'nobody'") {
		t.Fatalf("err = %v", err)
	}
}

func TestInviteStatus(t *testing.T) {
	pinNow(t)
	n := now()
	cases := []struct {
		inv  Invite
		want string
	}{
		{Invite{Created: n.Add(-1 * time.Hour)}, "pending, 6d left"},
		{Invite{Created: n.Add(-6*24*time.Hour - 12*time.Hour)}, "pending, 12h left"},
		{Invite{Created: n.Add(-7*24*time.Hour + 30*time.Minute)}, "pending, <1h left"},
		{Invite{Created: n.Add(-8 * 24 * time.Hour), Expired: true}, "EXPIRED"},
		{Invite{}, "pending"},
	}
	for _, c := range cases {
		if got := inviteStatus(c.inv); got != c.want {
			t.Errorf("inviteStatus(created %v) = %q, want %q", c.inv.Created, got, c.want)
		}
	}
}
