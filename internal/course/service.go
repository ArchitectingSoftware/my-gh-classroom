package course

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/gh"
)

var repoNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Service struct {
	GH    *gh.Client
	C     config.Classroom
	Apply bool
}

func New(c config.Classroom, apply bool) *Service { return &Service{GH: gh.New(), C: c, Apply: apply} }

func is404(err error) bool {
	t := err.Error()
	return strings.Contains(t, "404") || strings.Contains(t, "Not Found")
}
func (s *Service) TeamGet(team string) (map[string]any, bool, error) {
	var v map[string]any
	err := s.GH.JSON(&v, "api", fmt.Sprintf("orgs/%s/teams/%s", s.C.Organization, team))
	if err != nil && is404(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}
func (s *Service) EnsureTeam(team string) (map[string]any, error) {
	v, ok, err := s.TeamGet(team)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("team '%s' does not exist in %s", team, s.C.Organization)
	}
	return v, nil
}
func (s *Service) TeamMembers(team string) ([]map[string]any, error) {
	if _, err := s.EnsureTeam(team); err != nil {
		return nil, err
	}
	var v []map[string]any
	err := s.GH.JSON(&v, "api", "--paginate", fmt.Sprintf("orgs/%s/teams/%s/members", s.C.Organization, team))
	return v, err
}
func (s *Service) CreateTeam(team string) error {
	_, ok, err := s.TeamGet(team)
	if err != nil {
		return err
	}
	if ok {
		fmt.Printf("SKIP      team '%s' already exists in %s\n", team, s.C.Organization)
		return nil
	}
	if !s.Apply {
		fmt.Printf("DRY RUN   would create team '%s' in %s\n", team, s.C.Organization)
		return nil
	}
	_, err = s.GH.Run("api", "--method", "POST", fmt.Sprintf("orgs/%s/teams", s.C.Organization), "-f", "name="+team, "-f", "description=Course grading team for "+s.C.Organization, "-f", "privacy=closed")
	if err == nil {
		fmt.Printf("CREATED   team '%s' in %s\n", team, s.C.Organization)
	}
	return err
}
func (s *Service) AddTeamMember(team, user string) error {
	if _, err := s.EnsureTeam(team); err != nil {
		return err
	}
	u, err := s.UserGet(user)
	if err != nil {
		return err
	}
	login := fmt.Sprint(u["login"])
	ms, err := s.TeamMembers(team)
	if err != nil {
		return err
	}
	for _, m := range ms {
		if fmt.Sprint(m["login"]) == login {
			fmt.Printf("SKIP      %s is already a member of '%s'\n", login, team)
			return nil
		}
	}
	if !s.Apply {
		fmt.Printf("DRY RUN   would add %s to '%s'\n", login, team)
		return nil
	}
	_, err = s.GH.Run("api", "--method", "PUT", fmt.Sprintf("orgs/%s/teams/%s/memberships/%s", s.C.Organization, team, login), "-f", "role=member")
	if err == nil {
		fmt.Printf("ADDED     %s to '%s'\n", login, team)
	}
	return err
}
func (s *Service) RemoveTeamMember(team, user string) error {
	ms, err := s.TeamMembers(team)
	if err != nil {
		return err
	}
	found := false
	for _, m := range ms {
		if fmt.Sprint(m["login"]) == user {
			found = true
		}
	}
	if !found {
		fmt.Printf("SKIP      %s is not a member of '%s'\n", user, team)
		return nil
	}
	if !s.Apply {
		fmt.Printf("DRY RUN   would remove %s from '%s'\n", user, team)
		return nil
	}
	_, err = s.GH.Run("api", "--method", "DELETE", fmt.Sprintf("orgs/%s/teams/%s/memberships/%s", s.C.Organization, team, user))
	if err == nil {
		fmt.Printf("REMOVED   %s from '%s'\n", user, team)
	}
	return err
}

func (s *Service) UserGet(user string) (map[string]any, error) {
	var v map[string]any
	if err := s.GH.JSON(&v, "api", fmt.Sprintf("users/%s", user)); err != nil {
		return nil, err
	}
	if fmt.Sprint(v["login"]) == "" {
		return nil, fmt.Errorf("could not resolve GitHub user '%s'", user)
	}
	return v, nil
}
func (s *Service) RepoGet(repo string) (map[string]any, bool, error) {
	var v map[string]any
	err := s.GH.JSON(&v, "api", fmt.Sprintf("repos/%s/%s", s.C.Organization, repo))
	if err != nil && is404(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}
func normalizePermission(p string) string {
	if p == "write" {
		return "push"
	}
	return p
}
func (s *Service) TeamRepoPermission(slug, repo string) (string, bool, error) {
	var v map[string]any
	err := s.GH.JSON(&v, "api", "-H", "Accept: application/vnd.github.v3.repository+json", fmt.Sprintf("orgs/%s/teams/%s/repos/%s/%s", s.C.Organization, slug, s.C.Organization, repo))
	if err != nil && is404(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if r := fmt.Sprint(v["role_name"]); r != "" {
		return normalizePermission(r), true, nil
	}
	if p, ok := v["permissions"].(map[string]any); ok {
		for _, n := range []string{"admin", "maintain", "write", "push", "triage", "pull"} {
			if b, _ := p[n].(bool); b {
				return normalizePermission(n), true, nil
			}
		}
	}
	return "connected", true, nil
}

func (s *Service) ListRepos() ([]map[string]any, error) {
	var all []map[string]any
	for page := 1; ; page++ {
		var data []map[string]any
		err := s.GH.JSON(&data, "api", fmt.Sprintf("orgs/%s/repos?per_page=100&page=%d&type=all&sort=full_name&direction=asc", s.C.Organization, page))
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			break
		}
		all = append(all, data...)
		if len(data) < 100 {
			break
		}
	}
	return all, nil
}
func (s *Service) PendingInvitations(repo string) ([]map[string]any, error) {
	var v []map[string]any
	err := s.GH.JSON(&v, "api", "--paginate", fmt.Sprintf("repos/%s/%s/invitations", s.C.Organization, repo))
	return v, err
}
func permissionFromMap(v map[string]any) string {
	if v == nil {
		return ""
	}
	p := fmt.Sprint(v["permission"])
	return normalizePermission(p)
}

func studentReadme(name, github, url string) string {
	return fmt.Sprintf("# CS472 Student Repository\n\n**Student:** %s  \n**GitHub:** %s\n\nThis is your private repository for CS472. Use this same repository for the entire course, with a separate directory for each assignment.\n\nFor course repository instructions, Git/GitHub workflow, and submission guidance, see:\n\n%s\n", name, github, url)
}
func (s *Service) SetStudentProperties(repo, name, github string) error {
	payload := map[string]any{"properties": []map[string]string{{"property_name": "repo_type", "value": "student"}, {"property_name": "student_name", "value": name}, {"property_name": "github_id", "value": github}}}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.GH.RunInput(data, "api", "--method", "PATCH", fmt.Sprintf("repos/%s/%s/properties/values", s.C.Organization, repo))
	return err
}

func (s *Service) CreateStudentRepo(name, github, repoName string) (string, error) {
	if _, err := s.EnsureTeam(s.C.GraderTeam); err != nil {
		return "", err
	}
	u, err := s.UserGet(github)
	if err != nil {
		return "", err
	}
	actual := fmt.Sprint(u["login"])
	if repoName == "" {
		repoName = actual
	}
	if !repoNameRE.MatchString(repoName) {
		return "", fmt.Errorf("invalid repository name '%s'", repoName)
	}
	if existing, ok, err := s.RepoGet(repoName); err != nil {
		return "", err
	} else if ok {
		fmt.Printf("SKIP      %s already exists\n          %s\n", repoName, existing["html_url"])
		return "existing", nil
	}
	if !s.Apply {
		fmt.Printf("DRY RUN   would create private repo %s/%s\n          student: %s (%s)\n          add student collaborator with: %s\n          add team '%s' with: %s\n          initialize README with link to course-info\n          set custom properties: repo_type=student, student_name=%s, github_id=%s\n", s.C.Organization, repoName, name, actual, s.C.StudentPermission, s.C.GraderTeam, s.C.GraderPermission, name, actual)
		return "dry-run", nil
	}
	data, err := s.GH.Run("api", "--method", "POST", fmt.Sprintf("orgs/%s/repos", s.C.Organization), "-f", "name="+repoName, "-f", "description=CS472 student repository for "+name, "-F", "private=true", "-F", "has_issues=false", "-F", "has_projects=false", "-F", "has_wiki=false", "-F", "auto_init=true")
	if err != nil {
		return "", err
	}
	_ = data
	if _, err = s.GH.Run("api", "--method", "PUT", fmt.Sprintf("repos/%s/%s/collaborators/%s", s.C.Organization, repoName, actual), "-f", "permission="+s.C.StudentPermission); err != nil {
		return "", err
	}
	team, err := s.EnsureTeam(s.C.GraderTeam)
	if err != nil {
		return "", err
	}
	slug := fmt.Sprint(team["slug"])
	if _, err = s.GH.Run("api", "--method", "PUT", fmt.Sprintf("orgs/%s/teams/%s/repos/%s/%s", s.C.Organization, slug, s.C.Organization, repoName), "-f", "permission="+s.C.GraderPermission); err != nil {
		return "", err
	}
	var readme map[string]any
	if err := s.GH.JSON(&readme, "api", fmt.Sprintf("repos/%s/%s/contents/README.md", s.C.Organization, repoName)); err != nil {
		return "", err
	}
	sha := fmt.Sprint(readme["sha"])
	enc := base64.StdEncoding.EncodeToString([]byte(studentReadme(name, actual, s.C.CourseInfoURL)))
	if _, err = s.GH.Run("api", "--method", "PUT", fmt.Sprintf("repos/%s/%s/contents/README.md", s.C.Organization, repoName), "-f", "message=Initialize course README", "-f", "content="+enc, "-f", "sha="+sha); err != nil {
		return "", err
	}
	if err := s.SetStudentProperties(repoName, name, actual); err != nil {
		return "", err
	}
	fmt.Printf("CREATED   %s (%s)\n          repo: https://github.com/%s/%s\n          student access: %s\n          grader team: %s (%s)\n          invitation: GitHub will notify the student if an invitation is pending\n", name, actual, s.C.Organization, repoName, s.C.StudentPermission, s.C.GraderTeam, s.C.GraderPermission)
	return "created", nil
}

func nameOr(name, github string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return github
}

func (s *Service) Batch(path, githubColumn, nameColumn, repoColumn string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open CSV %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	headers, err := r.Read()
	if err != nil {
		return fmt.Errorf("could not read CSV header: %w", err)
	}

	index := make(map[string]int, len(headers))
	for i, h := range headers {
		index[strings.TrimSpace(h)] = i
	}
	for _, required := range []string{githubColumn, nameColumn} {
		if _, ok := index[required]; !ok {
			return fmt.Errorf("CSV column '%s' was not found; available columns: %s", required, strings.Join(headers, ", "))
		}
	}
	if repoColumn != "" {
		if _, ok := index[repoColumn]; !ok {
			return fmt.Errorf("CSV column '%s' was not found; available columns: %s", repoColumn, strings.Join(headers, ", "))
		}
	}

	fmt.Printf("Processing students from %s\n\n", path)
	counts := map[string]int{"created": 0, "existing": 0, "dry-run": 0, "error": 0, "blank": 0}
	row := 1
	for {
		rec, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		row++
		if readErr != nil {
			counts["error"]++
			fmt.Printf("ERROR     row %d: could not read row: %v\n\n", row, readErr)
			continue
		}

		cell := func(column string) string {
			return strings.TrimSpace(rec[index[column]])
		}
		github := cell(githubColumn)
		name := cell(nameColumn)
		repo := ""
		if repoColumn != "" {
			repo = cell(repoColumn)
		}

		if github == "" {
			counts["blank"]++
			fmt.Printf("BLANK     row %d: no GitHub username\n\n", row)
			continue
		}

		result, err := s.CreateStudentRepo(nameOr(name, github), github, repo)
		if err != nil {
			counts["error"]++
			fmt.Printf("ERROR     row %d (%s): %v\n\n", row, nameOr(name, github), err)
			continue
		}
		counts[result]++
		fmt.Println()
	}

	fmt.Println("Batch summary")
	fmt.Printf("  Created:  %d\n", counts["created"])
	fmt.Printf("  Existing: %d\n", counts["existing"])
	fmt.Printf("  Dry run:  %d\n", counts["dry-run"])
	fmt.Printf("  Errors:   %d\n", counts["error"])
	fmt.Printf("  Blank:    %d\n", counts["blank"])
	return nil
}

func (s *Service) StudentInfo(github string) error {
	u, err := s.UserGet(github)
	if err != nil {
		return err
	}
	login := fmt.Sprint(u["login"])
	repo, ok, err := s.RepoGet(login)
	if err != nil {
		return err
	}
	fmt.Printf("Student:      %v\nGitHub:       %s\nGitHub URL:   %v\n", display(u["name"], login), login, u["html_url"])
	if !ok {
		fmt.Printf("Repository:   NOT CREATED (%s/%s)\n", s.C.Organization, login)
		return nil
	}
	fmt.Printf("Repository:   %v\nPrivate:      %v\nDefault:      %v\n", repo["html_url"], repo["private"], repo["default_branch"])
	var perm map[string]any
	if err := s.GH.JSON(&perm, "api", fmt.Sprintf("repos/%s/%s/collaborators/%s/permission", s.C.Organization, login, login)); err == nil && perm != nil {
		fmt.Printf("Student perm: %s\n", permissionFromMap(perm))
	}
	if t, ok, err := s.TeamGet(s.C.GraderTeam); err == nil && ok {
		p, c, _ := s.TeamRepoPermission(fmt.Sprint(t["slug"]), login)
		if c {
			fmt.Printf("Grader team:  %s (%s)\n", s.C.GraderTeam, p)
		} else {
			fmt.Printf("Grader team:  %s (NOT CONNECTED)\n", s.C.GraderTeam)
		}
	}
	invs, _ := s.PendingInvitations(login)
	if len(invs) == 0 {
		fmt.Println("Pending inv.: none")
	} else {
		fmt.Printf("Pending inv.: %d\n", len(invs))
		for _, inv := range invs {
			who := inv["invitee"].(map[string]any)
			fmt.Printf("              %v\n", who["login"])
		}
	}
	return nil
}
func display(v any, fallback string) any {
	if v == nil || fmt.Sprint(v) == "<nil>" || fmt.Sprint(v) == "" {
		return fallback
	}
	return v
}

func (s *Service) StudentList() error {
	repos, err := s.ListRepos()
	if err != nil {
		return err
	}
	count := 0
	for _, r := range repos {
		if fmt.Sprint(r["name"]) != s.C.CourseInfoRepo {
			count++
		}
	}
	fmt.Printf("Student repositories in %s (%d):\n", s.C.Organization, count)
	sort.Slice(repos, func(i, j int) bool { return fmt.Sprint(repos[i]["name"]) < fmt.Sprint(repos[j]["name"]) })
	for _, r := range repos {
		if fmt.Sprint(r["name"]) == s.C.CourseInfoRepo {
			continue
		}
		props, _ := r["custom_properties"].(map[string]any)
		name := fmt.Sprint(props["student_name"])
		if name == "<nil>" {
			name = ""
		}
		id := fmt.Sprint(props["github_id"])
		if id == "<nil>" || id == "" {
			id = fmt.Sprint(r["name"])
		}
		fmt.Printf("  %-25s %-25s %s\n", name, id, r["html_url"])
	}
	return nil
}
func (s *Service) StudentFind(q string) error {
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return fmt.Errorf("search query cannot be empty")
	}
	repos, err := s.ListRepos()
	if err != nil {
		return err
	}
	fmt.Printf("Student search: '%s'\n", q)
	matches := 0
	for _, r := range repos {
		if fmt.Sprint(r["name"]) == s.C.CourseInfoRepo {
			continue
		}
		props, _ := r["custom_properties"].(map[string]any)
		fields := []string{fmt.Sprint(r["name"]), fmt.Sprint(props["student_name"]), fmt.Sprint(props["github_id"])}
		hit := false
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), needle) {
				hit = true
				break
			}
		}
		if hit {
			matches++
			if matches == 1 {
				fmt.Printf("Matches (%d):\n", 1)
			}
			fmt.Printf("  %-25s %-25s %s\n", fmt.Sprint(props["student_name"]), fmt.Sprint(props["github_id"]), r["html_url"])
		}
	}
	if matches == 0 {
		fmt.Println("No matching student repositories found.")
	}
	return nil
}

func (s *Service) PropertiesList() error {
	var v []map[string]any
	if err := s.GH.JSON(&v, "api", fmt.Sprintf("orgs/%s/properties/schema", s.C.Organization)); err != nil {
		return err
	}
	if len(v) == 0 {
		fmt.Println("No custom properties defined.")
		return nil
	}
	for _, p := range v {
		fmt.Printf("%-18s %-14s %v\n", p["property_name"], p["value_type"], p["description"])
	}
	return nil
}
func (s *Service) PropertiesSetup() error {
	names := map[string]string{"repo_type": "Identifies course student repositories", "student_name": "Student name for course administration", "github_id": "Student GitHub username"}
	var schema []map[string]any
	if err := s.GH.JSON(&schema, "api", fmt.Sprintf("orgs/%s/properties/schema", s.C.Organization)); err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, p := range schema {
		existing[fmt.Sprint(p["property_name"])] = true
	}
	for name, desc := range names {
		if existing[name] {
			fmt.Printf("SKIP      custom property '%s' already exists\n", name)
			continue
		}
		if !s.Apply {
			fmt.Printf("DRY RUN   would create custom property '%s'\n", name)
			continue
		}
		body := map[string]any{"value_type": "string", "required": false, "description": desc}
		data, _ := json.Marshal(body)
		var out map[string]any
		if err := s.GH.JSONInput(&out, data, "api", "--method", "PUT", fmt.Sprintf("orgs/%s/properties/schema/%s", s.C.Organization, name)); err != nil {
			return err
		}
		fmt.Printf("CREATED   custom property '%s'\n", name)
	}
	return nil
}

func (s *Service) Doctor(active string) error {
	if _, err := s.GH.Run("auth", "status"); err != nil {
		return err
	}
	var u map[string]any
	if err := s.GH.JSON(&u, "api", "user"); err != nil {
		return err
	}
	fmt.Printf("GitHub Course Admin - Doctor\n\nClassroom:       %s\nOrganization:    %s\nAuthenticated:   OK (%v)\n", active, s.C.Organization, u["login"])
	var org map[string]any
	if err := s.GH.JSON(&org, "api", fmt.Sprintf("orgs/%s", s.C.Organization)); err != nil {
		return err
	}
	fmt.Printf("Organization:    OK (%v)\n", org["login"])
	t, ok, err := s.TeamGet(s.C.GraderTeam)
	if err != nil {
		return err
	}
	if ok {
		fmt.Printf("Grader team:     OK (%v)\nTeam permission: %s\n", t["name"], s.C.GraderPermission)
	} else {
		fmt.Printf("Grader team:     NOT CREATED (%s)\n", s.C.GraderTeam)
	}
	fmt.Printf("Course info URL: %s\n", s.C.CourseInfoURL)
	return nil
}
