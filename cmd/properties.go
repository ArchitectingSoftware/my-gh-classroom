package cmd

import "github.com/spf13/cobra"

func propertiesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "properties", Short: "Manage organization custom repository properties"}
	cmd.AddCommand(&cobra.Command{Use: "setup", RunE: func(_ *cobra.Command, _ []string) error { return svc.PropertiesSetup() }}, &cobra.Command{Use: "list", RunE: func(_ *cobra.Command, _ []string) error { return svc.PropertiesList() }})
	return cmd
}
