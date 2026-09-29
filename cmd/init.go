package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var home bool
	c := &cobra.Command{
		Use:   "init [ALIAS]",
		Short: "Create a starter config.json",
		Long: `Create a starter config.json with one placeholder classroom, which is
also the default classroom. ALIAS names it (default "` + config.ExampleAlias + `").

The file is written to ./config.json, or to ~/.mgc/config.json with
--home, or to the path given with --config. An existing file is never
overwritten: rename or delete it first.

Like other mgc commands this is a dry run unless --apply is given; the
dry run prints the file it would write.

mgc looks for its config in this order: --config, $MGC_CONFIG,
./config.json, ~/.mgc/config.json, then the OS config directory.`,
		Example: `  mgc init                      # preview ./config.json
  mgc --apply init cs472        # write ./config.json with classroom cs472
  mgc --apply init cs472 --home # write ~/.mgc/config.json instead`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, a []string) error {
			alias := config.ExampleAlias
			if len(a) == 1 {
				alias = a[0]
			}
			target := config.LocalPath
			switch {
			case configPath != "" && home:
				return fmt.Errorf("use either --config or --home, not both")
			case configPath != "":
				target = configPath
			case home:
				if target = config.HomePath(); target == "" {
					return fmt.Errorf("could not determine your home directory; use --config PATH")
				}
			}
			scaffold := config.Scaffold(alias)
			if err := config.ValidateClassroom(alias, scaffold.Classrooms[alias]); err != nil {
				return err
			}
			if _, err := os.Stat(target); err == nil {
				return existsErr(target)
			}

			if !apply {
				data, err := config.Encode(scaffold)
				if err != nil {
					return err
				}
				fmt.Printf("DRY RUN   would create %s:\n\n%s\n", target, data)
				shadowNote(target)
				fmt.Println("Re-run with --apply to write it.")
				return nil
			}
			if err := config.Create(target, scaffold); err != nil {
				if errors.Is(err, config.ErrExists) {
					return existsErr(target)
				}
				return err
			}
			fmt.Printf("CREATED   %s with classroom '%s' (the default)\n", target, alias)
			shadowNote(target)
			fmt.Println("\nNext:")
			fmt.Println("  1. edit the YOUR_* placeholders (organization, course_info_repo, course_info_url)")
			fmt.Println("  2. mgc classroom verify     # checks the org, team, and course-info repo on GitHub")
			fmt.Println("  3. mgc doctor")
			return nil
		},
	}
	c.Flags().BoolVar(&home, "home", false, "Write ~/.mgc/config.json instead of ./config.json")
	return c
}

func existsErr(p string) error {
	return fmt.Errorf("%s already exists; mgc init never overwrites a config. Rename or delete it to generate a new one", p)
}

// shadowNote warns when a config file with higher precedence would be
// used instead of the one being created.
func shadowNote(target string) {
	if configPath != "" {
		return // written to an explicit path; the user will pass --config
	}
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	if active := config.ResolvePath(""); active != "" && abs(active) != abs(target) {
		if _, err := os.Stat(active); err == nil {
			fmt.Printf("NOTE      %s takes precedence and will be used instead; use --config %s to select this one\n", active, target)
			return
		}
	}
	if env := os.Getenv("MGC_CONFIG"); env != "" && abs(env) != abs(target) {
		fmt.Printf("NOTE      $MGC_CONFIG (%s) takes precedence and will be used instead\n", env)
	}
}
