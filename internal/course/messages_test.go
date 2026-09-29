package course

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/roster"
)

func TestSignature(t *testing.T) {
	cases := []struct {
		names []string
		want  string
	}{
		{nil, "The CS281 teaching team"},
		{[]string{"  ", ""}, "The CS281 teaching team"},
		{[]string{"Dr. Brian Mitchell"}, "Dr. Brian Mitchell"},
		{[]string{"Prof. Mitchell", "Prof. Jones"}, "Prof. Mitchell and Prof. Jones"},
		{[]string{"A", "B", "C"}, "A, B, and C"},
	}
	for _, c := range cases {
		s, _, _ := newService(t, false)
		s.C.Instructors = c.names
		if got := s.signature(); got != c.want {
			t.Errorf("signature(%q) = %q, want %q", c.names, got, c.want)
		}
	}
}

func TestSignatureFallsBackToOrganization(t *testing.T) {
	s, _, _ := newService(t, false)
	s.C.CourseName = ""
	if got := s.signature(); got != "The "+org+" teaching team" {
		t.Errorf("got %q", got)
	}
}

func TestGreeting(t *testing.T) {
	cases := map[[2]string]string{
		{"Jane Smith", "jsmith42"}:  "Hi Jane,",
		{"Smith, Jane", "jsmith42"}: "Hi Jane,",
		{"jsmith42", "jsmith42"}:    "Hi,",
		{"", "jsmith42"}:            "Hi,",
		{"Madonna", ""}:             "Hi Madonna,",
	}
	for in, want := range cases {
		if got := greeting(in[0], in[1]); got != want {
			t.Errorf("greeting(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestStudentInvitesMessages(t *testing.T) {
	pinNow(t)
	s, f, _ := newService(t, false)
	s.C.Instructors = []string{"Dr. Brian Mitchell"}
	inviteRoutes(f)
	msgs, err := s.Invites("")
	if err != nil {
		t.Fatal(err)
	}
	// Zed and cat4 are pending; Amy's invitation expired, so no message.
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2: %+v", len(msgs), msgs)
	}
	var zed Message
	for _, m := range msgs {
		if strings.Contains(m.To, "Amy") {
			t.Errorf("expired invitation got a message: %+v", m)
		}
		if strings.Contains(m.To, "Zed") {
			zed = m
		}
	}
	if zed.To != "Zed Zulu (GitHub: zed1)" || zed.Subject != "CS281: accept your GitHub repository invitation" {
		t.Errorf("header: %+v", zed)
	}
	for _, want := range []string{"Hi Zed,", "sign in to GitHub as zed1", "https://github.com/" + org + "/zed1/invitations", "expires on " + time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC).Local().Format("Monday, January 2"), "Dr. Brian Mitchell"} {
		if !strings.Contains(zed.Body, want) {
			t.Errorf("body missing %q:\n%s", want, zed.Body)
		}
	}
}

func TestStudentInviteMessageForOne(t *testing.T) {
	pinNow(t)
	s, f, _ := newService(t, false)
	inviteRoutes(f)
	msgs, err := s.Invites("zed1")
	if err != nil || len(msgs) != 1 || !strings.Contains(msgs[0].Body, "The CS281 teaching team") {
		t.Fatalf("msgs %+v err %v", msgs, err)
	}
	msgs, _ = s.Invites("amy2") // expired
	if len(msgs) != 0 {
		t.Errorf("expired invitation got a message: %+v", msgs)
	}
}

func TestTeamInvitesMessages(t *testing.T) {
	pinNow(t)
	s, f, _ := newService(t, false)
	teamRoutes(f, `[]`, teamInvitesJSON+``)
	msgs, err := s.TeamInvitesReport("graders", "")
	if err != nil {
		t.Fatal(err)
	}
	// Only ta-bo is usable (ta-cy expired, dee@ failed).
	if len(msgs) != 1 || msgs[0].To != "GitHub: ta-bo" {
		t.Fatalf("msgs = %+v", msgs)
	}
	for _, want := range []string{"CS281 teaching team on GitHub", "sign in to GitHub as ta-bo", orgInviteURL} {
		if !strings.Contains(msgs[0].Body, want) {
			t.Errorf("body missing %q:\n%s", want, msgs[0].Body)
		}
	}
}

func TestTeamInviteMessageEmailOnly(t *testing.T) {
	s, _, _ := newService(t, false)
	m := s.teamInviteMessage(Invite{Email: "dee@drexel.edu", URL: orgInviteURL})
	if m.To != "GitHub: dee@drexel.edu" || !strings.Contains(m.Body, "GitHub account for dee@drexel.edu") {
		t.Errorf("%+v", m)
	}
}

func TestImportMessagesOnlyForStudentFixableProblems(t *testing.T) {
	s, f, _ := newService(t, false)
	s.C.Instructors = []string{"Prof. Mitchell", "Prof. Jones"}
	importFixture(f, existingRepos).
		fail("GET", "users/ghost", errNotFound).
		fail("GET", "users/flaky", errWithArgs("gh: Server Error (HTTP 502)")).
		user("zed")

	sum, err := s.ImportStudents([]roster.Student{
		stu(2, "Blank Id", ""),
		stu(3, "Bad Id", "jane@drexel.edu"),
		stu(4, "Ghost Person", "ghost"),
		stu(5, "Taken", "taken"),   // repo conflict: instructor's problem
		stu(6, "Flaky", "flaky"),   // GitHub error: instructor's problem
		stu(7, "Zed", "zed"),       // fine
		stu(8, "Zed Again", "zed"), // duplicate: skipped, no message
	}, importMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Errors != 5 || len(sum.Messages) != 3 {
		t.Fatalf("errors %d, messages %d: %+v", sum.Errors, len(sum.Messages), sum.Messages)
	}
	want := []struct{ to, text string }{
		{"Blank Id", "your GitHub username was missing from the survey"},
		{"Bad Id", `"jane@drexel.edu" isn't a valid GitHub username`},
		{"Ghost Person", `there is no GitHub account named "ghost"`},
	}
	for i, w := range want {
		m := sum.Messages[i]
		if m.To != w.to || !strings.Contains(m.Body, w.text) || m.Subject != "CS281: please check your GitHub username" {
			t.Errorf("message %d = %+v, want to %q containing %q", i, m, w.to, w.text)
		}
		if !strings.Contains(m.Body, "Prof. Mitchell and Prof. Jones") || !strings.Contains(m.Body, "We couldn't create your CS281 repository because\n") {
			t.Errorf("message %d body:\n%s", i, m.Body)
		}
	}
	if !strings.HasPrefix(sum.Messages[2].Body, "Hi Ghost,") {
		t.Errorf("greeting:\n%s", sum.Messages[2].Body)
	}
}

func TestWriteMessages(t *testing.T) {
	var b bytes.Buffer
	WriteMessages(&b, []Message{{To: "A", Subject: "S1", Body: "body one\n"}, {To: "B", Subject: "S2", Body: "body two\n"}})
	o := b.String()
	if strings.Count(o, strings.Repeat("=", 72)) != 3 || !strings.Contains(o, "To:      A\nSubject: S1\n\nbody one\n") {
		t.Errorf("output:\n%s", o)
	}
	b.Reset()
	WriteMessages(&b, nil)
	if b.Len() != 0 {
		t.Errorf("no messages should write nothing, got %q", b.String())
	}
}

func TestMessageLinesFitEmail(t *testing.T) {
	pinNow(t)
	s, _, _ := newService(t, false)
	s.C.Instructors = []string{"Dr. Brian Mitchell"}
	inv := Invite{Name: "Jane Smith", Login: "jsmith42", URL: "https://github.com/" + org + "/jsmith42/invitations", Created: now()}
	msgs := []Message{
		s.studentInviteMessage(inv),
		s.teamInviteMessage(Invite{Login: "ta-bo", URL: orgInviteURL, Created: now()}),
	}
	for _, p := range []string{problemBlank, problemInvalid, problemNotFound} {
		m, _ := s.rosterProblemMessage(stu(2, "Jane Smith", "js"), p)
		msgs = append(msgs, m)
	}
	for _, m := range msgs {
		for _, line := range strings.Split(m.Body, "\n") {
			if len(line) > 80 {
				t.Errorf("line longer than 80 chars in %q:\n%s", m.Subject, line)
			}
		}
	}
}
