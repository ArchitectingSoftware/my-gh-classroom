package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/course"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/roster"
	"github.com/spf13/cobra"
)

// resultsFile is where each import run writes its report. It is
// overwritten on every run.
var resultsFile = "import-results.txt"

func importCmd() *cobra.Command {
	var (
		number       int
		nameColumn   string
		githubColumn string
		repair       bool
		message      bool
		keepCase     bool
	)
	c := &cobra.Command{
		Use:   "import ROSTER_CSV",
		Short: "Create student repositories from a CSV roster",
		Long: `Create a repository for every student in a CSV roster who does not
already have one.

The CSV's header row must include these columns (exact spelling):

  Name        the student's name, e.g. "Jane Smith"
  GitHub-ID   the student's GitHub username, e.g. "jsmith42"

If your file uses different headers, name them with --name-column and
--github-column. Any other columns are ignored, and column order does
not matter, so an LMS export (such as a Canvas survey "Student Analysis"
CSV) can be used as-is. The file is checked for both columns before
GitHub is contacted.

Each repository is named <repo_prefix><GitHub ID>, where repo_prefix is
set per classroom in config.json (default: no prefix). Names are
lowercased unless the classroom sets "repo_name_case": "preserve" or
--keep-case is given, in which case the prefix and the GitHub ID keep
their case (the ID as GitHub reports it, not as typed in the CSV).

This is an upsert: students who already have a repository are skipped
and nothing about their repository is changed, so the same file can be
imported repeatedly. With --repair, existing repositories are checked
instead and anything missing is fixed: student access, grader-team
access, custom properties, and the course README (only while the
repository still has nothing but GitHub's initial commit). Student work
is never modified. Like other mgc commands this is a dry run unless
--apply is given.

A report is printed as students are processed and also written to
` + resultsFile + ` in the current directory (overwritten each run).`,
		Example: `  mgc classroom import roster.csv                   # dry run, whole file
  mgc classroom import roster.csv -n 2              # dry run, first 2 students
  mgc --apply classroom import roster.csv           # create repositories
  mgc --apply classroom import roster.csv --repair  # create, and fix incomplete repos
  mgc classroom import export.csv --name-column Student --github-column "GitHub Username"
  mgc classroom import roster.csv --message         # plus notes for students to fix their username
  mgc -c cs281 --apply classroom import roster.csv  # a non-default classroom`,
		Args: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				fmt.Fprintln(os.Stderr, "WARNING: no roster CSV file specified.")
				fmt.Fprintln(os.Stderr)
				cmd.SetOut(os.Stderr)
				_ = cmd.Usage()
				return errUsageShown
			case 1:
				return nil
			default:
				return fmt.Errorf("expected one CSV file, got %d arguments", len(args))
			}
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if number < 0 {
				return fmt.Errorf("--number must be a positive number of records, got %d", number)
			}
			if keepCase {
				svc.KeepCase = true
			}
			path := args[0]
			students, err := roster.ReadFile(path, roster.Options{Limit: number, NameColumn: nameColumn, GitHubColumn: githubColumn})
			if err != nil {
				if errors.Is(err, roster.ErrMissingColumns) {
					return fmt.Errorf("%s: %w\nuse --name-column/--github-column if your headers differ; see: mgc classroom import --help", path, err)
				}
				return fmt.Errorf("%s: %w", path, err)
			}
			if len(students) == 0 {
				return fmt.Errorf("%s contains no student records", path)
			}

			f, err := os.Create(resultsFile)
			if err != nil {
				return fmt.Errorf("could not create %s: %w", resultsFile, err)
			}
			defer f.Close()

			sum, err := svc.ImportStudents(students, course.ImportMeta{
				File:         path,
				Classroom:    activeClassroom,
				Limit:        number,
				NameColumn:   nameColumn,
				GitHubColumn: githubColumn,
				Repair:       repair,
				Started:      time.Now(),
			}, f)
			fmt.Printf("\nResults written to %s\n", resultsFile)
			if err != nil {
				return err
			}
			if message {
				if err := emitMessages(sum.Messages, "no students need to fix their GitHub username"); err != nil {
					return err
				}
			}
			if sum.Errors > 0 {
				return fmt.Errorf("%d of %d students had errors; see %s", sum.Errors, sum.Processed, resultsFile)
			}
			return nil
		},
	}
	c.Flags().IntVarP(&number, "number", "n", 0, "Process only the first N students in the file (default: all)")
	c.Flags().StringVar(&nameColumn, "name-column", roster.NameColumn, "CSV column containing student names")
	c.Flags().StringVar(&githubColumn, "github-column", roster.GitHubColumn, "CSV column containing GitHub usernames")
	c.Flags().BoolVar(&repair, "repair", false, "Check existing repositories and fix anything missing instead of skipping them")
	c.Flags().BoolVar(&keepCase, "keep-case", false, "Keep the case of repo_prefix and GitHub IDs in repository names instead of lowercasing them")
	c.Flags().BoolVar(&message, "message", false, "Also write a ready-to-paste note for each student whose GitHub username is blank, invalid, or not found (saved to "+messagesFile+")")
	return c
}
