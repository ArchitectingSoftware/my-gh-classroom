package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
		// version and init work anywhere, with or without a config file.
		if cmd.Name() == "version" || cmd.Name() == "init" {
			return nil
		}
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
			return fmt.Errorf("no classroom selected and no default classroom is configured; use --classroom/-c or set a default")
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
	if err := rejectLegacyArgs(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	rootCmd.SetArgs(os.Args[1:])

	if err := rootCmd.Execute(); err != nil {
		if !errors.Is(err, errUsageShown) {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
		}
		os.Exit(1)
	}
}

// legacyFlags maps retired single-dash spellings to their replacements.
// Without this check pflag would parse them as clusters of shorthand flags
// ("-cr" as -c r, "-apply" as -a pply) and report confusing errors, or
// worse, silently treat "-cr" as -c with a value of "r".
var legacyFlags = map[string]string{
	"-apply": "--apply or -a",
	"-cr":    "--classroom or -c",
}

// rejectLegacyArgs reports the first retired flag spelling in args.
func rejectLegacyArgs(in []string) error {
	for _, a := range in {
		if a == "--" {
			break
		}
		name := a
		if i := strings.IndexByte(a, '='); i > 0 {
			name = a[:i]
		}
		if repl, ok := legacyFlags[name]; ok {
			return fmt.Errorf("%s is no longer supported; use %s", name, repl)
		}
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config.json (default: $MGC_CONFIG, ./config.json, ~/.mgc/config.json, or the OS config dir)")
	rootCmd.PersistentFlags().BoolVarP(&apply, "apply", "a", false, "Actually perform a mutating operation; otherwise mutating commands are dry-run")
	rootCmd.PersistentFlags().StringVarP(&classroomAlias, "classroom", "c", "", "Classroom alias to use; defaults to configured default classroom")
	rootCmd.AddCommand(initCmd(), doctorCmd(), teamCmd(), studentCmd(), propertiesCmd(), classroomCmd(), versionCmd())
	v, _, _, _ := buildVersion()
	rootCmd.Version = v
	rootCmd.SetVersionTemplate("mgc {{.Version}}\n")
}
