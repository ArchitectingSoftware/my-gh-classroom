package cmd

import (
	"fmt"
	"net/url"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/gh"
	"github.com/spf13/cobra"
)

func classroomCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "classroom", Short: "Manage configured classrooms"}

	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		aliases := cfg.ClassroomAliases()
		if len(aliases) == 0 {
			fmt.Println("No classrooms configured.")
			fmt.Println("WARNING: No default classroom is configured.")
			return nil
		}
		fmt.Println("Classrooms:")
		for _, alias := range aliases {
			marker := " "
			if alias == cfg.DefaultClassroom {
				marker = "*"
			}
			fmt.Printf("  %s %s\n", marker, alias)
		}
		if cfg.DefaultClassroom == "" {
			fmt.Println("WARNING: No default classroom is configured.")
		}
		return nil
	}}

	create := &cobra.Command{Use: "create ALIAS", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		alias := a[0]
		if _, ok := cfg.Classrooms[alias]; ok {
			return fmt.Errorf("classroom '%s' already exists", alias)
		}
		if err := config.ValidateClassroom(alias, config.Classroom{Organization: "placeholder", GraderTeam: "graders", GraderPermission: "push", StudentPermission: "push", CourseInfoRepo: "placeholder", CourseInfoURL: "https://github.com/placeholder/placeholder"}); err != nil {
			return err
		}
		cl := config.Classroom{
			Organization:      "YOUR_GITHUB_ORGANIZATION",
			GraderTeam:        "graders",
			GraderPermission:  "push",
			StudentPermission: "push",
			CourseInfoRepo:    "YOUR_COURSE_INFO_REPO",
			CourseInfoURL:     "https://github.com/YOUR_GITHUB_ORGANIZATION/YOUR_COURSE_INFO_REPO",
		}
		if !apply {
			fmt.Printf("DRY RUN   would add classroom '%s' to %s\n", alias, configPath)
			fmt.Printf("          organization: %s\n", cl.Organization)
			fmt.Printf("          course-info: %s\n", cl.CourseInfoRepo)
			return nil
		}
		cfg.Classrooms[alias] = cl
		if err := config.Save(configPath, cfg); err != nil {
			return err
		}
		fmt.Printf("CREATED   classroom '%s' in %s\n", alias, configPath)
		fmt.Println("          Edit its placeholder values before using it.")
		return nil
	}}

	remove := &cobra.Command{Use: "delete ALIAS", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		alias := a[0]
		if _, ok := cfg.Classrooms[alias]; !ok {
			return fmt.Errorf("classroom '%s' does not exist", alias)
		}
		if alias == cfg.DefaultClassroom {
			return fmt.Errorf("cannot delete default classroom '%s'; set another default first", alias)
		}
		if !apply {
			fmt.Printf("DRY RUN   would remove classroom '%s' from %s\n", alias, configPath)
			fmt.Println("          GitHub organizations and repositories will not be modified")
			return nil
		}
		delete(cfg.Classrooms, alias)
		if err := config.Save(configPath, cfg); err != nil {
			return err
		}
		fmt.Printf("DELETED   classroom '%s' from %s\n", alias, configPath)
		return nil
	}}

	defaultCmd := &cobra.Command{Use: "default [ALIAS]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		if len(a) == 0 {
			if cfg.DefaultClassroom == "" {
				fmt.Println("Default classroom: none")
			} else {
				fmt.Printf("Default classroom: %s\n", cfg.DefaultClassroom)
			}
			return nil
		}
		alias := a[0]
		if _, ok := cfg.Classrooms[alias]; !ok {
			return fmt.Errorf("classroom '%s' does not exist", alias)
		}
		if !apply {
			fmt.Printf("DRY RUN   would set default classroom to '%s'\n", alias)
			return nil
		}
		cfg.DefaultClassroom = alias
		if err := config.Save(configPath, cfg); err != nil {
			return err
		}
		fmt.Printf("UPDATED   default classroom: %s\n", alias)
		return nil
	}}

	verify := &cobra.Command{Use: "verify [ALIAS]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		alias := classroomAlias
		if alias == "" {
			alias = cfg.DefaultClassroom
		}
		if len(a) == 1 {
			alias = a[0]
		}
		if alias == "" {
			return fmt.Errorf("no classroom specified and no default classroom is configured")
		}
		cl, ok := cfg.Classrooms[alias]
		if !ok {
			return fmt.Errorf("classroom '%s' does not exist", alias)
		}
		fmt.Printf("Classroom:        %s\nOrganization:     %s\n", alias, cl.Organization)
		if err := config.ValidateClassroom(alias, cl); err != nil {
			fmt.Printf("Configuration:    FAIL (%v)\n", err)
			return err
		}
		fmt.Println("Configuration:    OK")
		g := gh.New()
		var org map[string]any
		if err := gh.JSON(g, &org, "api", fmt.Sprintf("orgs/%s", cl.Organization)); err != nil {
			return err
		}
		fmt.Printf("GitHub org:        OK (%v)\n", org["login"])
		var repo map[string]any
		if err := gh.JSON(g, &repo, "api", fmt.Sprintf("repos/%s/%s", cl.Organization, cl.CourseInfoRepo)); err != nil {
			return err
		}
		fmt.Printf("Course info repo:  OK (%v)\n", repo["name"])
		var team map[string]any
		if err := gh.JSON(g, &team, "api", fmt.Sprintf("orgs/%s/teams/%s", cl.Organization, cl.GraderTeam)); err != nil {
			return err
		}
		fmt.Printf("Grader team:       OK (%v)\n", team["name"])
		u, _ := url.Parse(cl.CourseInfoURL)
		fmt.Printf("Course info URL:   OK (%s)\n", u.Host)
		return nil
	}}

	cmd.AddCommand(list, create, remove, defaultCmd, verify)
	return cmd
}
