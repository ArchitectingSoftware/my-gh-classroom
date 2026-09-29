package course

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/gh"
)

// InviteLifetime is how long GitHub keeps a repository invitation open.
// See https://github.blog/changelog/2020-02-05-self-expiring-repository-and-organization-invitations/
const InviteLifetime = 7 * 24 * time.Hour

// Invite is one pending repository invitation.
type Invite struct {
	ID      string // invitation id (organization invitations)
	Name    string // student name from custom properties, or the login/email
	Login   string // invited GitHub account
	Email   string // invited email address, for email-only invitations
	Repo    string // repository, for repository invitations
	URL     string // page where the invitee accepts
	Created time.Time
	Expired bool
	Failed  string // failure reason reported by GitHub, if any
}

// Expires is when the invitation stops working (zero if unknown).
func (i Invite) Expires() time.Time {
	if i.Created.IsZero() {
		return time.Time{}
	}
	return i.Created.Add(InviteLifetime)
}

// now is replaceable in tests.
var now = time.Now

func inviteURL(org, repo string, inv map[string]any) string {
	if u := str(inv["html_url"]); u != "" {
		return u
	}
	return fmt.Sprintf("https://github.com/%s/%s/invitations", org, repo)
}

// repoInvites converts a repository's pending invitations.
func (s *Service) repoInvites(r map[string]any) ([]Invite, error) {
	repo := str(r["name"])
	invs, err := s.PendingInvitations(repo)
	if err != nil {
		return nil, err
	}
	name := repoProps(r)["student_name"]
	var out []Invite
	for _, inv := range invs {
		login := invitee(inv)
		created, _ := time.Parse(time.RFC3339, str(inv["created_at"]))
		expired, _ := inv["expired"].(bool)
		if !created.IsZero() && now().After(created.Add(InviteLifetime)) {
			expired = true
		}
		n := name
		if n == "" {
			n = login
		}
		out = append(out, Invite{ID: idString(inv["id"]), Name: n, Login: login, Repo: repo, URL: inviteURL(s.C.Organization, repo, inv), Created: created, Expired: expired})
	}
	return out, nil
}

// cancelRepoInvite deletes a repository invitation. An invitation that is
// already gone is not an error.
func (s *Service) cancelRepoInvite(repo, id string) error {
	_, err := s.GH.Run("api", "--method", "DELETE", fmt.Sprintf("repos/%s/%s/invitations/%s", s.C.Organization, repo, id))
	if gh.IsNotFound(err) {
		return nil
	}
	return err
}

// candidateRepos are repositories that may hold student invitations:
// student repositories plus untagged ones (e.g. from an interrupted
// import). Repositories tagged as something else, and the course-info
// repository, are excluded.
func (s *Service) candidateRepos(repos []map[string]any) []map[string]any {
	var out []map[string]any
	for _, r := range repos {
		t := repoProps(r)["repo_type"]
		if str(r["name"]) == s.C.CourseInfoRepo || (t != "" && t != "student") {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Invites prints pending invitations. With an empty query it reports
// every student repository in the organization; otherwise it reports the
// one student matching a GitHub ID or repository name.
func (s *Service) Invites(query string) error {
	repos, err := s.ListRepos()
	if err != nil {
		return err
	}
	if strings.TrimSpace(query) != "" {
		return s.studentInvite(repos, strings.TrimSpace(query))
	}

	var all []Invite
	var failed []string
	for _, r := range s.candidateRepos(repos) {
		invs, err := s.repoInvites(r)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", str(r["name"]), concise(err)))
			continue
		}
		all = append(all, invs...)
	}
	sort.Slice(all, func(i, j int) bool {
		if a, b := strings.ToLower(all[i].Name), strings.ToLower(all[j].Name); a != b {
			return a < b
		}
		return all[i].Login < all[j].Login
	})

	expired := 0
	for _, i := range all {
		if i.Expired {
			expired++
		}
	}
	s.printf("Pending repository invitations in %s (%d", s.C.Organization, len(all))
	if expired > 0 {
		s.printf(", %d expired", expired)
	}
	s.printf("):\n")
	if len(all) == 0 {
		s.println("  none: every invited student has accepted")
	} else {
		s.println()
		s.printf("  %-28s %-22s %-18s %s\n", "STUDENT", "GITHUB ID", "STATUS", "ACCEPT AT")
		for _, i := range all {
			url := i.URL
			if i.Expired {
				url = "(expired: invite again)"
			}
			s.printf("  %-28s %-22s %-18s %s\n", truncate(i.Name, 28), truncate(i.Login, 22), inviteStatus(i), url)
		}
	}
	s.println()
	s.println("Students must be signed in to the invited GitHub account to accept.")
	if expired > 0 {
		s.println("EXPIRED invitations no longer work. Re-invite with:")
		s.println("  mgc --apply classroom import ROSTER.csv --repair     (everyone)")
		s.println("  mgc student invites GITHUB_ID                         (shows the command for one student)")
	}
	if len(failed) > 0 {
		s.printf("\nCould not read invitations for %d repositories:\n", len(failed))
		for _, f := range failed {
			s.printf("  %s\n", f)
		}
		return fmt.Errorf("could not read invitations for %d repositories", len(failed))
	}
	return nil
}

// studentInvite reports the invitation state for one student.
func (s *Service) studentInvite(repos []map[string]any, query string) error {
	r, ok := findStudentRepo(repos, query, s.C.RepoPrefix)
	if !ok {
		return fmt.Errorf("no repository found for '%s' in %s", query, s.C.Organization)
	}
	repo := str(r["name"])
	name := repoProps(r)["student_name"]
	login := repoProps(r)["github_id"]
	if login == "" {
		login = query
	}
	if name == "" {
		name = login
	}
	invs, err := s.repoInvites(r)
	if err != nil {
		return err
	}
	s.printf("Student:      %s\nGitHub:       %s\nRepository:   %s\n", name, login, str(r["html_url"]))
	if len(invs) == 0 {
		collab, err := s.IsCollaborator(repo, login)
		if err != nil {
			return err
		}
		if collab {
			s.println("Invitation:   none pending; the student already has access")
		} else {
			s.println("Invitation:   none pending, and the student does not have access")
			s.println("              invite them with: mgc --apply classroom import ROSTER.csv --repair")
		}
		return nil
	}
	for _, i := range invs {
		s.printf("Invitation:   %s\n", inviteStatus(i))
		if !i.Created.IsZero() {
			s.printf("Invited:      %s\n", i.Created.Local().Format("2006-01-02 15:04"))
			s.printf("Expires:      %s\n", i.Expires().Local().Format("2006-01-02 15:04"))
		}
		if i.Login != "" && !strings.EqualFold(i.Login, login) {
			s.printf("Invitee:      %s\n", i.Login)
		}
		if i.Expired {
			s.println("Accept at:    (expired; the student must be invited again)")
			s.printf("Re-invite:    mgc --apply student create --name %q --github %s --repo %s\n", name, login, repo)
			continue
		}
		s.printf("Accept at:    %s\n", i.URL)
		s.println("              (sign in to GitHub as " + i.Login + " first)")
	}
	return nil
}

// inviteStatus is a short human description, e.g. "pending, 5d left".
func inviteStatus(i Invite) string {
	if i.Failed != "" {
		return "FAILED"
	}
	if i.Expired {
		return "EXPIRED"
	}
	if i.Created.IsZero() {
		return "pending"
	}
	left := i.Expires().Sub(now())
	switch {
	case left >= 48*time.Hour:
		return fmt.Sprintf("pending, %dd left", int(left.Hours()/24))
	case left >= time.Hour:
		return fmt.Sprintf("pending, %dh left", int(left.Hours()))
	default:
		return "pending, <1h left"
	}
}
