package course

import (
	"fmt"
	"io"
	"strings"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/roster"
)

// Message is a ready-to-paste note for one person, produced by --message.
// mgc never sends it; the instructor copies it into email or Canvas.
type Message struct {
	To      string // e.g. "Jane Smith (GitHub: jsmith42)"
	Subject string
	Body    string
}

// signature signs messages with the configured instructors, falling back
// to "The <course> teaching team".
func (s *Service) signature() string {
	var names []string
	for _, n := range s.C.Instructors {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	switch len(names) {
	case 0:
		return fmt.Sprintf("The %s teaching team", courseLabel(s.C))
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

// greeting addresses someone by first name when a real name is known.
// "Smith, Jane" is handled as well as "Jane Smith".
func greeting(name, login string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, login) {
		return "Hi,"
	}
	if last, first, ok := strings.Cut(name, ","); ok && strings.TrimSpace(first) != "" && strings.TrimSpace(last) != "" {
		name = strings.TrimSpace(first)
	}
	return "Hi " + strings.Fields(name)[0] + ","
}

func recipient(name, login string) string {
	if name == "" || strings.EqualFold(name, login) {
		return "GitHub: " + login
	}
	return fmt.Sprintf("%s (GitHub: %s)", name, login)
}

func expiryLine(i Invite) string {
	if i.Created.IsZero() {
		return ""
	}
	return fmt.Sprintf("\nThe invitation expires on %s.\n", i.Expires().Local().Format("Monday, January 2"))
}

// studentInviteMessage asks a student to accept their repository invitation.
func (s *Service) studentInviteMessage(i Invite) Message {
	course := courseLabel(s.C)
	return Message{
		To:      recipient(i.Name, i.Login),
		Subject: course + ": accept your GitHub repository invitation",
		Body: fmt.Sprintf("%s\n\nYour %s repository is ready. Accept the invitation here\n(sign in to GitHub as %s first):\n\n%s\n%s\n%s\n",
			greeting(i.Name, i.Login), course, i.Login, i.URL, expiryLine(i), s.signature()),
	}
}

// teamInviteMessage asks a TA or grader to accept their organization invitation.
func (s *Service) teamInviteMessage(i Invite) Message {
	course := courseLabel(s.C)
	who := i.Login
	signIn := fmt.Sprintf("(sign in to GitHub as %s first)", i.Login)
	if who == "" {
		who = i.Email
		signIn = "(sign in to the GitHub account for " + i.Email + " first)"
	}
	return Message{
		To:      recipient("", who),
		Subject: course + ": accept your GitHub organization invitation",
		Body: fmt.Sprintf("Hi,\n\nYou've been added to the %s teaching team on GitHub.\nAccept the invitation here %s:\n\n%s\n%s\n%s\n",
			course, signIn, i.URL, expiryLine(i), s.signature()),
	}
}

// Reasons a roster row failed that the student can fix themselves.
const (
	problemBlank    = "blank"
	problemInvalid  = "invalid"
	problemNotFound = "not-found"
)

// rosterProblemMessage asks a student to correct the GitHub username they
// submitted. It returns false for problems the student cannot fix.
func (s *Service) rosterProblemMessage(st roster.Student, problem string) (Message, bool) {
	course := courseLabel(s.C)
	var why string
	switch problem {
	case problemBlank:
		why = "your GitHub username was missing from the survey"
	case problemInvalid:
		why = fmt.Sprintf("%q isn't a valid GitHub username", st.RawID)
	case problemNotFound:
		why = fmt.Sprintf("there is no GitHub account named %q", st.RawID)
	default:
		return Message{}, false
	}
	to := st.Name
	if to == "" {
		to = fmt.Sprintf("CSV row %d", st.Row)
	}
	return Message{
		To:      to,
		Subject: course + ": please check your GitHub username",
		Body: fmt.Sprintf("%s\n\nWe couldn't create your %s repository because\n%s.\n\nPlease resubmit the survey with your exact GitHub username (not your\nuniversity ID or email). You can see it by signing in at https://github.com.\n\n%s\n",
			greeting(st.Name, ""), course, why, s.signature()),
	}, true
}

// WriteMessages prints messages as copy-ready blocks.
func WriteMessages(w io.Writer, msgs []Message) {
	for _, m := range msgs {
		fmt.Fprintf(w, "%s\nTo:      %s\nSubject: %s\n\n%s", strings.Repeat("=", 72), m.To, m.Subject, m.Body)
	}
	if len(msgs) > 0 {
		fmt.Fprintln(w, strings.Repeat("=", 72))
	}
}
