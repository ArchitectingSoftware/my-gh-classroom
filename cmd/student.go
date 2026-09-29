package cmd

import (
	"github.com/spf13/cobra"
)

func studentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "student", Short: "Manage student repositories"}
	create := &cobra.Command{Use: "create", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		name, _ := cmd.Flags().GetString("name")
		github, _ := cmd.Flags().GetString("github")
		repo, _ := cmd.Flags().GetString("repo")
		if keep, _ := cmd.Flags().GetBool("keep-case"); keep {
			svc.KeepCase = true
		}
		_, err := svc.CreateStudentRepo(name, github, repo)
		return err
	}}
	create.Flags().String("name", "", "Student name")
	create.Flags().String("github", "", "Student GitHub username")
	create.Flags().String("repo", "", "Repository name, used exactly as given (default: the classroom's repo_prefix + GitHub ID)")
	create.Flags().Bool("keep-case", false, "Keep the case of repo_prefix and the GitHub ID in the repository name instead of lowercasing it")
	_ = create.MarkFlagRequired("name")
	_ = create.MarkFlagRequired("github")
	list := &cobra.Command{Use: "list", RunE: func(_ *cobra.Command, _ []string) error { return svc.StudentList() }}
	info := &cobra.Command{Use: "info GITHUB_ID", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error { return svc.StudentInfo(a[0]) }}
	find := &cobra.Command{Use: "find QUERY", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error { return svc.StudentFind(a[0]) }}
	invites := &cobra.Command{
		Use:   "invites [GITHUB_ID|REPO]",
		Short: "Show pending repository invitations and the URL to accept each one",
		Long: `Show pending repository invitations and the page where each student can
accept theirs, for when a student has lost or ignored GitHub's email
(GitHub does not re-send it).

With no argument, every pending invitation in the classroom's
organization is listed by student name. With a GitHub ID or repository
name, just that student is shown.

Students accept at https://github.com/<org>/<repo>/invitations while
signed in to the invited GitHub account. Invitations expire after 7
days; expired ones are marked EXPIRED and the student must be invited
again.`,
		Example: `  mgc student invites                     # everyone with a pending invitation
  mgc student invites jsmith42            # one student
  mgc student invites --message           # plus a ready-to-paste note for each`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			q := ""
			if len(a) == 1 {
				q = a[0]
			}
			msgs, err := svc.Invites(q)
			if want, _ := c.Flags().GetBool("message"); want {
				if e := emitMessages(msgs, "no pending invitations that can still be accepted"); e != nil && err == nil {
					err = e
				}
			}
			return err
		},
	}
	invites.Flags().Bool("message", false, "Also write a ready-to-paste message for each student (saved to "+messagesFile+")")
	cmd.AddCommand(create, list, info, find, invites)
	return cmd
}
