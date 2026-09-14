package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func teamCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "team", Short: "Manage organization teams"}
	create := &cobra.Command{Use: "create [name]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		if len(a) > 0 {
			name = a[0]
		}
		return svc.CreateTeam(name)
	}}
	info := &cobra.Command{Use: "info [name]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		if len(a) > 0 {
			name = a[0]
		}
		v, ok, e := svc.TeamGet(name)
		if e != nil {
			return e
		}
		if !ok {
			return fmt.Errorf("team '%s' does not exist", name)
		}
		fmt.Printf("Team:         %v\nOrganization: %s\nSlug:         %v\nPrivacy:      %v\nDescription:  %v\nURL:          %v\n", v["name"], activeConfig.Organization, v["slug"], v["privacy"], v["description"], v["html_url"])
		return nil
	}}
	list := &cobra.Command{Use: "list [name]", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		if len(a) > 0 {
			name = a[0]
		}
		m, e := svc.TeamMembers(name)
		if e != nil {
			return e
		}
		fmt.Printf("Members of %s/%s (%d):\n", activeConfig.Organization, name, len(m))
		for _, x := range m {
			fmt.Println(" ", x["login"])
		}
		return nil
	}}
	add := &cobra.Command{Use: "add [name] USERNAME", Args: cobra.RangeArgs(1, 2), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		user := a[0]
		if len(a) == 2 {
			name = a[0]
			user = a[1]
		}
		return svc.AddTeamMember(name, user)
	}}
	rem := &cobra.Command{Use: "remove [name] USERNAME", Args: cobra.RangeArgs(1, 2), RunE: func(_ *cobra.Command, a []string) error {
		name := activeConfig.GraderTeam
		user := a[0]
		if len(a) == 2 {
			name = a[0]
			user = a[1]
		}
		return svc.RemoveTeamMember(name, user)
	}}
	cmd.AddCommand(create, info, list, add, rem)
	return cmd
}
