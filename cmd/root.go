package cmd

import (
	"fmt"
	"os"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/course"
	"github.com/spf13/cobra"
)

var (
	configPath      string
	apply           bool
	classroomAlias  string
	cfg             config.Config
	svc             *course.Service
	activeClassroom string
	activeConfig    config.Classroom
)

var rootCmd = &cobra.Command{
	Use:   "gh-course-admin",
	Short: "GitHub administration utility for programming courses",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return err
		}
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		// Classroom management commands load the configuration but do not require
		// a selected classroom, except verify which takes an optional alias.
		if cmd.Parent() != nil && cmd.Parent().Name() == "classroom" {
			return nil
		}

		alias := classroomAlias
		if alias == "" {
			alias = cfg.DefaultClassroom
		}
		if alias == "" {
			fmt.Println("WARNING: No default classroom is configured.")
			return fmt.Errorf("no classroom selected and no default classroom is configured; use --classroom/-cr or set a default")
		}
		cl, err := cfg.Classroom(alias)
		if err != nil {
			return err
		}
		activeClassroom = alias
		activeConfig = cl
		svc = course.New(cl, apply)
		fmt.Printf("Classroom: %s\nOrganization: %s\n\n", alias, cl.Organization)
		return nil
	},
}

func Execute() {
	// pflag only permits a single-character shorthand. We intentionally support
	// the more readable legacy forms -cr and -apply by normalizing them to
	// their long-form equivalents before Cobra/pflag parses the arguments.
	args := make([]string, len(os.Args))
	copy(args, os.Args)
	for i := range args {
		switch args[i] {
		case "-cr":
			args[i] = "--classroom"
		case "-apply":
			args[i] = "--apply"
		}
	}
	rootCmd.SetArgs(args[1:])

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "config.json", "Path to config.json")
	rootCmd.PersistentFlags().BoolVarP(&apply, "apply", "a", false, "Actually perform a mutating operation; otherwise mutating commands are dry-run")
	rootCmd.PersistentFlags().StringVar(&classroomAlias, "classroom", "", "Classroom alias to use; defaults to configured default classroom")
	rootCmd.AddCommand(doctorCmd(), teamCmd(), studentCmd(), propertiesCmd(), classroomCmd())
}
