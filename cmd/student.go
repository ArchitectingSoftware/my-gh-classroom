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
		_, err := svc.CreateStudentRepo(name, github, repo)
		return err
	}}
	create.Flags().String("name", "", "Student name")
	create.Flags().String("github", "", "Student GitHub username")
	create.Flags().String("repo", "", "Optional repository name")
	_ = create.MarkFlagRequired("name")
	_ = create.MarkFlagRequired("github")
	batch := &cobra.Command{Use: "create-batch CSV", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		gc, _ := cmd.Flags().GetString("github-column")
		nc, _ := cmd.Flags().GetString("name-column")
		rc, _ := cmd.Flags().GetString("repo-column")
		return svc.Batch(a[0], gc, nc, rc)
	}}
	batch.Flags().String("github-column", "GitHub Username", "CSV column containing GitHub usernames")
	batch.Flags().String("name-column", "Student", "CSV column containing student names")
	batch.Flags().String("repo-column", "", "Optional CSV column containing repository names")
	list := &cobra.Command{Use: "list", RunE: func(_ *cobra.Command, _ []string) error { return svc.StudentList() }}
	info := &cobra.Command{Use: "info GITHUB_ID", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error { return svc.StudentInfo(a[0]) }}
	find := &cobra.Command{Use: "find QUERY", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error { return svc.StudentFind(a[0]) }}
	cmd.AddCommand(create, batch, list, info, find)
	return cmd
}
