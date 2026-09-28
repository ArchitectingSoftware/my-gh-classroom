package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/canvas"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/course"
	"github.com/spf13/cobra"
)

// resultsFile is where each import run writes its report. It is
// overwritten on every run.
var resultsFile = "import-results.txt"

func importCmd() *cobra.Command {
	var number int
	c := &cobra.Command{
		Use:   "import CANVAS_CSV",
		Short: "Create student repositories from a Canvas export",
		Long: `Create a repository for every student in a Canvas export who does not
already have one. The CSV must contain "Name" and "GitHub-ID" columns;
other columns are ignored.

This is an upsert: students who already have a repository are skipped
and nothing about their repository is changed, so the same file can be
imported repeatedly. Like other mgc commands it is a dry run unless
-apply is given.

A report is printed as students are processed and also written to
` + resultsFile + ` in the current directory (overwritten each run).`,
		Example: `  mgc classroom import Canvas-Export.csv            # dry run, whole file
  mgc classroom import Canvas-Export.csv -n 2       # dry run, first 2 students
  mgc -apply classroom import Canvas-Export.csv     # create repositories
  mgc -cr cs281 -apply classroom import roster.csv  # a non-default classroom`,
		Args: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				fmt.Fprintln(os.Stderr, "WARNING: no Canvas CSV file specified.")
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
			path := args[0]
			students, err := canvas.ReadFile(path, number)
			if err != nil {
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
				File:      path,
				Classroom: activeClassroom,
				Limit:     number,
				Started:   time.Now(),
			}, f)
			fmt.Printf("\nResults written to %s\n", resultsFile)
			if err != nil {
				return err
			}
			if sum.Errors > 0 {
				return fmt.Errorf("%d of %d students had errors; see %s", sum.Errors, sum.Processed, resultsFile)
			}
			return nil
		},
	}
	c.Flags().IntVarP(&number, "number", "n", 0, "Process only the first N students in the file (default: all)")
	return c
}
