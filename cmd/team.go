package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func teamCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "team", Short: "Manage organization teams"}
	create := &cobra.Command{Use: "create [name]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		if len(a) > 0 {
			name = a[0]
		}
		return svc.CreateTeam(name)
	}}
	info := &cobra.Command{Use: "info [name]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		if len(a) > 0 {
			name = a[0]
		}
		v, ok, e := svc.TeamGet(name)
		if e != nil {
			return e
		}
		if !ok {
			return fmt.Errorf("team '%s' does not exist", name)
		}
		fmt.Printf("Team:         %v\nOrganization: %s\nSlug:         %v\nPrivacy:      %v\nDescription:  %v\nURL:          %v\n", v["name"], activeConfig.Organization, v["slug"], v["privacy"], v["description"], v["html_url"])
		return nil
	}}
	list := &cobra.Command{Use: "list [name]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		if len(a) > 0 {
			name = a[0]
		}
		return svc.TeamList(name)
	}}
	add := &cobra.Command{Use: "add [name] USERNAME", Args: cobra.RangeArgs(1, 2), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		user := a[0]
		if len(a) == 2 {
			name = a[0]
			user = a[1]
		}
		return svc.AddTeamMember(name, user)
	}}
	rem := &cobra.Command{Use: "remove [name] USERNAME", Args: cobra.RangeArgs(1, 2), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		user := a[0]
		if len(a) == 2 {
			name = a[0]
			user = a[1]
		}
		return svc.RemoveTeamMember(name, user)
	}}
	var inviteTeam string
	invites := &cobra.Command{
		Use:   "invites [USERNAME]",
		Short: "Show pending team invitations and the URL to accept them",
		Long: `Show people who were added to the grader team but have not yet accepted
GitHub's organization invitation, and where to accept it, for when a TA
or grader has lost the email (GitHub does not re-send it).

With no argument, every pending invitation to the team is listed. With a
GitHub username (or invited email), just that person is shown.

Invitations are accepted at https://github.com/orgs/<org>/invitation
while signed in to the invited GitHub account. They expire after 7 days;
expired ones are marked and the person must be added again.`,
		Example: `  mgc team invites              # everyone who has not accepted yet
  mgc team invites ta-alice     # one person
  mgc team invites --message    # plus a ready-to-paste note for each
  mgc team invites --team staff # a team other than grader_team`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			name := activeConfig.GraderTeam
			if inviteTeam != "" {
				name = inviteTeam
			}
			who := ""
			if len(a) == 1 {
				who = a[0]
			}
			msgs, err := svc.TeamInvitesReport(name, who)
			if want, _ := c.Flags().GetBool("message"); want && err == nil {
				err = emitMessages(msgs, "no pending invitations that can still be accepted")
			}
			return err
		},
	}
	invites.Flags().Bool("message", false, "Also write a ready-to-paste message for each person (saved to "+messagesFile+")")
	invites.Flags().StringVar(&inviteTeam, "team", "", "Team to report on (default: the classroom's grader_team)")
	rem.Short = "Remove a member from a team, or cancel their pending invitation"
	cmd.AddCommand(create, info, list, add, rem, invites)
	return cmd
}
