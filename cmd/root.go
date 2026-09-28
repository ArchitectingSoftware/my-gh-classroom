package cmd

import (
	"errors"
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

// errUsageShown signals that a command already printed a warning and its
// usage; Execute exits non-zero without printing anything further.
var errUsageShown = errors.New("usage shown")

var rootCmd = &cobra.Command{
	Use:   "mgc",
	Short: "GitHub administration utility for programming courses",
	// Execute prints errors itself; without these Cobra would print each
	// error a second time followed by the full usage text.
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		var err error
		configPath = config.ResolvePath(configPath)
		cfg, err = config.Load(configPath)
		if err != nil {
			return err
		}
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		// Classroom management commands load the configuration but do not require
		// a selected classroom (verify takes an optional alias). The exception is
		// import, which provisions students into the active classroom.
		if cmd.Parent() != nil && cmd.Parent().Name() == "classroom" && cmd.Name() != "import" {
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
		alias, cl, err := cfg.ClassroomWithAlias(alias)
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
	rootCmd.SetArgs(normalizeArgs(os.Args[1:]))

	if err := rootCmd.Execute(); err != nil {
		if !errors.Is(err, errUsageShown) {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
		}
		os.Exit(1)
	}
}

// normalizeArgs rewrites -cr and -apply to their long forms. Arguments after
// a bare "--" are left untouched.
func normalizeArgs(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	for i, a := range out {
		if a == "--" {
			break
		}
		switch a {
		case "-cr":
			out[i] = "--classroom"
		case "-apply":
			out[i] = "--apply"
		}
	}
	return out
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config.json (default: $MGC_CONFIG, ./config.json, or ~/.config/mgc/config.json)")
	rootCmd.PersistentFlags().BoolVarP(&apply, "apply", "a", false, "Actually perform a mutating operation; otherwise mutating commands are dry-run")
	rootCmd.PersistentFlags().StringVar(&classroomAlias, "classroom", "", "Classroom alias to use; defaults to configured default classroom")
	rootCmd.AddCommand(doctorCmd(), teamCmd(), studentCmd(), propertiesCmd(), classroomCmd())
}
