package course

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/gh"
)

// Adding someone who is not yet in the organization to a team sends them
// an organization invitation. Their team membership stays "pending" until
// they accept it, and GitHub's team member list does not include them.

// OrgInviteURL is the page where an invited TA or grader accepts an
// organization invitation (while signed in to the invited account).
func (s *Service) OrgInviteURL() string {
	return fmt.Sprintf("https://github.com/orgs/%s/invitation", s.C.Organization)
}

// idString renders a numeric JSON id without float formatting
// (fmt.Sprint(float64(123456789)) would give "1.23456789e+08").
func idString(v any) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return str(v)
}

// TeamInvites returns pending invitations to a team, sorted by login.
func (s *Service) TeamInvites(team string) ([]Invite, error) {
	if _, err := s.EnsureTeam(team); err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := s.json(&raw, "api", "--paginate", fmt.Sprintf("orgs/%s/teams/%s/invitations", s.C.Organization, team)); err != nil {
		return nil, err
	}
	var out []Invite
	for _, inv := range raw {
		login, email := str(inv["login"]), str(inv["email"])
		name := login
		if name == "" {
			name = email
		}
		created, _ := time.Parse(time.RFC3339, str(inv["created_at"]))
		failed := str(inv["failed_reason"])
		if failed == "" && str(inv["failed_at"]) != "" {
			failed = "failed"
		}
		out = append(out, Invite{
			ID:      idString(inv["id"]),
			Name:    name,
			Login:   login,
			Email:   email,
			URL:     s.OrgInviteURL(),
			Created: created,
			Expired: !created.IsZero() && now().After(created.Add(InviteLifetime)),
			Failed:  failed,
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// cancelOrgInvite cancels an organization invitation. One that is already
// gone is not an error.
func (s *Service) cancelOrgInvite(id string) error {
	_, err := s.GH.Run("api", "--method", "DELETE", fmt.Sprintf("orgs/%s/invitations/%s", s.C.Organization, id))
	if gh.IsNotFound(err) {
		return nil
	}
	return err
}

// findInvite matches a username or email, case-insensitively.
func findInvite(invs []Invite, who string) (Invite, bool) {
	for _, i := range invs {
		if (i.Login != "" && strings.EqualFold(i.Login, who)) || (i.Email != "" && strings.EqualFold(i.Email, who)) {
			return i, true
		}
	}
	return Invite{}, false
}

// usable reports whether the invitation can still be accepted.
func (i Invite) usable() bool { return !i.Expired && i.Failed == "" }

// TeamList prints active members and pending invitations.
func (s *Service) TeamList(team string) error {
	ms, err := s.TeamMembers(team)
	if err != nil {
		return err
	}
	invs, err := s.TeamInvites(team)
	if err != nil {
		return err
	}
	s.printf("Members of %s/%s (%d active, %d invited):\n", s.C.Organization, team, len(ms), len(invs))
	for _, m := range ms {
		s.printf("  %s\n", str(m["login"]))
	}
	for _, i := range invs {
		s.printf("  %-24s invited, %s\n", i.Name, strings.TrimPrefix(inviteStatus(i), "pending, "))
	}
	if len(invs) > 0 {
		s.printf("\nInvited people accept at %s\n(see: mgc team invites)\n", s.OrgInviteURL())
	}
	return nil
}

// TeamInvitesReport prints pending invitations to a team, or the status of
// one person when who is non-empty.
func (s *Service) TeamInvitesReport(team, who string) error {
	invs, err := s.TeamInvites(team)
	if err != nil {
		return err
	}
	if who = strings.TrimSpace(who); who != "" {
		return s.teamInviteOne(team, who, invs)
	}

	unusable := 0
	for _, i := range invs {
		if !i.usable() {
			unusable++
		}
	}
	s.printf("Pending invitations to %s/%s (%d", s.C.Organization, team, len(invs))
	if unusable > 0 {
		s.printf(", %d expired or failed", unusable)
	}
	s.printf("):\n")
	if len(invs) == 0 {
		s.println("  none: everyone added to the team has accepted")
		return nil
	}
	s.println()
	s.printf("  %-30s %-18s %s\n", "GITHUB ID / EMAIL", "STATUS", "ACCEPT AT")
	for _, i := range invs {
		url := i.URL
		if !i.usable() {
			url = "(invite again)"
		}
		s.printf("  %-30s %-18s %s\n", truncate(i.Name, 30), inviteStatus(i), url)
	}
	s.println()
	s.println("They must be signed in to the invited GitHub account to accept.")
	if unusable > 0 {
		s.println("EXPIRED or FAILED invitations no longer work. Re-invite with:")
		s.println("  mgc --apply team add USERNAME")
	}
	return nil
}

func (s *Service) teamInviteOne(team, who string, invs []Invite) error {
	if i, ok := findInvite(invs, who); ok {
		s.printf("Team:         %s/%s\nInvitee:      %s\nInvitation:   %s\n", s.C.Organization, team, i.Name, inviteStatus(i))
		if i.Failed != "" && i.Failed != "failed" {
			s.printf("Reason:       %s\n", i.Failed)
		}
		if !i.Created.IsZero() {
			s.printf("Invited:      %s\n", i.Created.Local().Format("2006-01-02 15:04"))
			s.printf("Expires:      %s\n", i.Expires().Local().Format("2006-01-02 15:04"))
		}
		if !i.usable() {
			s.println("Accept at:    (no longer valid; the person must be invited again)")
			if i.Login != "" {
				s.printf("Re-invite:    mgc --apply team add %s\n", i.Login)
			}
			return nil
		}
		s.printf("Accept at:    %s\n", i.URL)
		s.printf("              (sign in to GitHub as %s first)\n", i.Name)
		return nil
	}
	ms, err := s.TeamMembers(team)
	if err != nil {
		return err
	}
	if login, ok := findMember(ms, who); ok {
		s.printf("%s is already an active member of %s/%s; nothing to accept.\n", login, s.C.Organization, team)
		return nil
	}
	s.printf("%s is not a member of %s/%s and has no pending invitation\n", who, s.C.Organization, team)
	s.printf("(an expired invitation may have been removed by GitHub). Add them with:\n  mgc --apply team add %s\n", who)
	return nil
}
