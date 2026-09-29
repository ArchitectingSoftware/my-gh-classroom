package cmd

import "github.com/spf13/cobra"

func doctorCmd() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check GitHub CLI, authentication, and active classroom", RunE: func(_ *cobra.Command, _ []string) error { return svc.Doctor(activeClassroom, configPath) }}
}
