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

// Result values returned by CreateStudentRepo and tallied by Batch.
const (
	ResultCreated  = "created"
	ResultExisting = "existing"
	ResultRepaired = "repaired"
	ResultDryRun   = "dry-run"
)

type Service struct {
	GH    gh.API
	C     config.Classroom
	Apply bool
	Out   io.Writer
}

func New(c config.Classroom, apply bool) *Service {
	return &Service{GH: gh.New(), C: c, Apply: apply, Out: os.Stdout}
}

func (s *Service) printf(format string, a ...any) { fmt.Fprintf(s.out(), format, a...) }
func (s *Service) println(a ...any)               { fmt.Fprintln(s.out(), a...) }
func (s *Service) out() io.Writer {
	if s.Out == nil {
		return os.Stdout
	}
	return s.Out
}

func (s *Service) json(out any, args ...string) error { return gh.JSON(s.GH, out, args...) }

// str converts a decoded JSON value to a string, mapping nil to "" rather
// than fmt.Sprint's "<nil>".
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// ---------------------------------------------------------------- teams

func (s *Service) TeamGet(team string) (map[string]any, bool, error) {
	var v map[string]any
	err := s.json(&v, "api", fmt.Sprintf("orgs/%s/teams/%s", s.C.Organization, team))
	if gh.IsNotFound(err) {
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
	err := s.json(&v, "api", "--paginate", fmt.Sprintf("orgs/%s/teams/%s/members", s.C.Organization, team))
	return v, err
}

func (s *Service) CreateTeam(team string) error {
	_, ok, err := s.TeamGet(team)
	if err != nil {
		return err
	}
	if ok {
		s.printf("SKIP      team '%s' already exists in %s\n", team, s.C.Organization)
		return nil
	}
	if !s.Apply {
		s.printf("DRY RUN   would create team '%s' in %s\n", team, s.C.Organization)
		return nil
	}
	_, err = s.GH.Run("api", "--method", "POST", fmt.Sprintf("orgs/%s/teams", s.C.Organization), "-f", "name="+team, "-f", "description=Course grading team for "+s.C.Organization, "-f", "privacy=closed")
	if err == nil {
		s.printf("CREATED   team '%s' in %s\n", team, s.C.Organization)
	}
	return err
}

// findMember returns the canonical login of user in members, matching
// case-insensitively as GitHub logins are.
func findMember(members []map[string]any, user string) (string, bool) {
	for _, m := range members {
		if login := str(m["login"]); strings.EqualFold(login, user) {
			return login, true
		}
	}
	return "", false
}

func (s *Service) AddTeamMember(team, user string) error {
	if _, err := s.EnsureTeam(team); err != nil {
		return err
	}
	u, err := s.UserGet(user)
	if err != nil {
		return err
	}
	login := str(u["login"])
	ms, err := s.TeamMembers(team)
	if err != nil {
		return err
	}
	if _, ok := findMember(ms, login); ok {
		s.printf("SKIP      %s is already a member of '%s'\n", login, team)
		return nil
	}
	if !s.Apply {
		s.printf("DRY RUN   would add %s to '%s'\n", login, team)
		return nil
	}
	_, err = s.GH.Run("api", "--method", "PUT", fmt.Sprintf("orgs/%s/teams/%s/memberships/%s", s.C.Organization, team, login), "-f", "role=member")
	if err == nil {
		s.printf("ADDED     %s to '%s'\n", login, team)
	}
	return err
}

func (s *Service) RemoveTeamMember(team, user string) error {
	ms, err := s.TeamMembers(team)
	if err != nil {
		return err
	}
	login, ok := findMember(ms, user)
	if !ok {
		s.printf("SKIP      %s is not a member of '%s'\n", user, team)
		return nil
	}
	if !s.Apply {
		s.printf("DRY RUN   would remove %s from '%s'\n", login, team)
		return nil
	}
	_, err = s.GH.Run("api", "--method", "DELETE", fmt.Sprintf("orgs/%s/teams/%s/memberships/%s", s.C.Organization, team, login))
	if err == nil {
		s.printf("REMOVED   %s from '%s'\n", login, team)
	}
	return err
}

// ---------------------------------------------------------------- users & repos

func (s *Service) UserGet(user string) (map[string]any, error) {
	var v map[string]any
	err := s.json(&v, "api", fmt.Sprintf("users/%s", user))
	if gh.IsNotFound(err) {
		return nil, fmt.Errorf("GitHub user '%s' does not exist", user)
	}
	if err != nil {
		return nil, err
	}
	if str(v["login"]) == "" {
		return nil, fmt.Errorf("could not resolve GitHub user '%s'", user)
	}
	return v, nil
}

func (s *Service) RepoGet(repo string) (map[string]any, bool, error) {
	var v map[string]any
	err := s.json(&v, "api", fmt.Sprintf("repos/%s/%s", s.C.Organization, repo))
	if gh.IsNotFound(err) {
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
	if p == "read" {
		return "pull"
	}
	return p
}

// TeamRepoPermission reports the team's permission on repo and whether the
// team is connected to it at all.
func (s *Service) TeamRepoPermission(slug, repo string) (string, bool, error) {
	var v map[string]any
	err := s.json(&v, "api", "-H", "Accept: application/vnd.github.v3.repository+json", fmt.Sprintf("orgs/%s/teams/%s/repos/%s/%s", s.C.Organization, slug, s.C.Organization, repo))
	if gh.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if r := str(v["role_name"]); r != "" {
		return normalizePermission(r), true, nil
	}
	if p, ok := v["permissions"].(map[string]any); ok {
		for _, n := range []string{"admin", "maintain", "push", "triage", "pull"} {
			if b, _ := p[n].(bool); b {
				return normalizePermission(n), true, nil
			}
		}
	}
	return "connected", true, nil
}

// IsCollaborator reports whether user is a direct collaborator on repo.
// GitHub returns 204 for collaborators and 404 otherwise (including users
// with a pending invitation).
func (s *Service) IsCollaborator(repo, user string) (bool, error) {
	_, err := s.GH.Run("api", fmt.Sprintf("repos/%s/%s/collaborators/%s", s.C.Organization, repo, user))
	if gh.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) ListRepos() ([]map[string]any, error) {
	var all []map[string]any
	for page := 1; ; page++ {
		var data []map[string]any
		err := s.json(&data, "api", fmt.Sprintf("orgs/%s/repos?per_page=100&page=%d&type=all&sort=full_name&direction=asc", s.C.Organization, page))
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
	err := s.json(&v, "api", "--paginate", fmt.Sprintf("repos/%s/%s/invitations", s.C.Organization, repo))
	return v, err
}

func invitee(inv map[string]any) string {
	who, _ := inv["invitee"].(map[string]any)
	return str(who["login"])
}

func (s *Service) hasPendingInvitation(repo, user string) (bool, error) {
	invs, err := s.PendingInvitations(repo)
	if err != nil {
		return false, err
	}
	for _, inv := range invs {
		if strings.EqualFold(invitee(inv), user) {
			return true, nil
		}
	}
	return false, nil
}

func permissionFromMap(v map[string]any) string {
	if v == nil {
		return ""
	}
	return normalizePermission(str(v["permission"]))
}

// repoProps extracts custom_properties from a repository object as strings.
func repoProps(r map[string]any) map[string]string {
	out := map[string]string{}
	props, _ := r["custom_properties"].(map[string]any)
	for k, v := range props {
		out[k] = str(v)
	}
	return out
}

func isStudentRepo(r map[string]any) bool {
	return repoProps(r)["repo_type"] == "student"
}

// ---------------------------------------------------------------- custom properties

// studentPropertyNames lists the custom properties mgc manages, in a stable order.
var studentPropertyNames = []struct{ Name, Description string }{
	{"repo_type", "Identifies course student repositories"},
	{"student_name", "Student name for course administration"},
	{"github_id", "Student GitHub username"},
}

func (s *Service) GetRepoProperties(repo string) (map[string]string, error) {
	var v []map[string]any
	if err := s.json(&v, "api", fmt.Sprintf("repos/%s/%s/properties/values", s.C.Organization, repo)); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, p := range v {
		out[str(p["property_name"])] = str(p["value"])
	}
	return out, nil
}

func (s *Service) SetRepoProperties(repo string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	props := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		props = append(props, map[string]string{"property_name": k, "value": values[k]})
	}
	data, err := json.Marshal(map[string]any{"properties": props})
	if err != nil {
		return err
	}
	_, err = s.GH.RunInput(data, "api", "--method", "PATCH", fmt.Sprintf("repos/%s/%s/properties/values", s.C.Organization, repo), "--input", "-")
	return err
}

func (s *Service) PropertiesList() error {
	var v []map[string]any
	if err := s.json(&v, "api", fmt.Sprintf("orgs/%s/properties/schema", s.C.Organization)); err != nil {
		return err
	}
	if len(v) == 0 {
		s.println("No custom properties defined.")
		return nil
	}
	for _, p := range v {
		s.printf("%-18s %-14s %s\n", str(p["property_name"]), str(p["value_type"]), str(p["description"]))
	}
	return nil
}

func (s *Service) PropertiesSetup() error {
	var schema []map[string]any
	if err := s.json(&schema, "api", fmt.Sprintf("orgs/%s/properties/schema", s.C.Organization)); err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, p := range schema {
		existing[str(p["property_name"])] = true
	}
	for _, p := range studentPropertyNames {
		if existing[p.Name] {
			s.printf("SKIP      custom property '%s' already exists\n", p.Name)
			continue
		}
		if !s.Apply {
			s.printf("DRY RUN   would create custom property '%s'\n", p.Name)
			continue
		}
		body := map[string]any{"value_type": "string", "required": false, "description": p.Description}
		data, _ := json.Marshal(body)
		var out map[string]any
		if err := gh.JSONInput(s.GH, &out, data, "api", "--method", "PUT", fmt.Sprintf("orgs/%s/properties/schema/%s", s.C.Organization, p.Name), "--input", "-"); err != nil {
			return err
		}
		s.printf("CREATED   custom property '%s'\n", p.Name)
	}
	return nil
}

// ---------------------------------------------------------------- student provisioning

func courseLabel(c config.Classroom) string {
	if strings.TrimSpace(c.CourseName) != "" {
		return c.CourseName
	}
	return c.Organization
}

func studentReadme(course, name, github, url string) string {
	return fmt.Sprintf("# %s Student Repository\n\n**Student:** %s  \n**GitHub:** %s\n\nThis is your private repository for %s. Use this same repository for the entire course, with a separate directory for each assignment.\n\nFor course repository instructions, Git/GitHub workflow, and submission guidance, see:\n\n%s\n", course, name, github, course, url)
}

// CreateStudentRepo provisions a student repository. If the repository
// already exists its content is never touched, but missing access grants and
// metadata are repaired so that a partially failed earlier run can be fixed
// by re-running. It returns one of the Result* constants.
func (s *Service) CreateStudentRepo(name, github, repoName string) (string, error) {
	team, err := s.EnsureTeam(s.C.GraderTeam)
	if err != nil {
		return "", err
	}
	slug := str(team["slug"])
	u, err := s.UserGet(github)
	if err != nil {
		return "", err
	}
	actual := str(u["login"])
	if repoName == "" {
		repoName = actual
	}
	if !repoNameRE.MatchString(repoName) {
		return "", fmt.Errorf("invalid repository name '%s'", repoName)
	}
	existing, ok, err := s.RepoGet(repoName)
	if err != nil {
		return "", err
	}
	if ok {
		return s.reconcileStudentRepo(existing, repoName, slug, name, actual)
	}
	if !s.Apply {
		s.printf("DRY RUN   would create private repo %s/%s\n          student: %s (%s)\n          add student collaborator with: %s\n          add team '%s' with: %s\n          initialize README with link to course-info\n          set custom properties: repo_type=student, student_name=%s, github_id=%s\n", s.C.Organization, repoName, name, actual, s.C.StudentPermission, s.C.GraderTeam, s.C.GraderPermission, name, actual)
		return ResultDryRun, nil
	}
	if err := s.createNew(slug, name, actual, repoName); err != nil {
		return "", err
	}
	s.printf("CREATED   %s (%s)\n          repo: https://github.com/%s/%s\n          student access: %s\n          grader team: %s (%s)\n          invitation: GitHub will notify the student if an invitation is pending\n", name, actual, s.C.Organization, repoName, s.C.StudentPermission, s.C.GraderTeam, s.C.GraderPermission)
	return ResultCreated, nil
}

// createNew creates and fully provisions a new student repository without
// printing anything. Callers are responsible for dry-run handling and
// for confirming the repository does not already exist.
func (s *Service) createNew(slug, name, login, repoName string) error {
	course := courseLabel(s.C)
	actual := login
	if _, err := s.GH.Run("api", "--method", "POST", fmt.Sprintf("orgs/%s/repos", s.C.Organization), "-f", "name="+repoName, "-f", fmt.Sprintf("description=%s student repository for %s", course, name), "-F", "private=true", "-F", "has_issues=false", "-F", "has_projects=false", "-F", "has_wiki=false", "-F", "auto_init=true"); err != nil {
		return err
	}
	var readme map[string]any
	if err := s.json(&readme, "api", fmt.Sprintf("repos/%s/%s/contents/README.md", s.C.Organization, repoName)); err != nil {
		return s.partial(repoName, name, actual, err)
	}
	enc := base64.StdEncoding.EncodeToString([]byte(studentReadme(course, name, actual, s.C.CourseInfoURL)))
	if _, err := s.GH.Run("api", "--method", "PUT", fmt.Sprintf("repos/%s/%s/contents/README.md", s.C.Organization, repoName), "-f", "message=Initialize course README", "-f", "content="+enc, "-f", "sha="+str(readme["sha"])); err != nil {
		return s.partial(repoName, name, actual, err)
	}
	if err := s.grantTeam(slug, repoName); err != nil {
		return s.partial(repoName, name, actual, err)
	}
	if err := s.grantStudent(repoName, actual); err != nil {
		return s.partial(repoName, name, actual, err)
	}
	if err := s.SetRepoProperties(repoName, map[string]string{"repo_type": "student", "student_name": name, "github_id": actual}); err != nil {
		return s.partial(repoName, name, actual, err)
	}
	return nil
}

func (s *Service) partial(repo, name, login string, err error) error {
	return fmt.Errorf("repository %s/%s was created but setup did not finish; repair it with: mgc -apply student create --name %q --github %s --repo %s: %w", s.C.Organization, repo, name, login, repo, err)
}

func (s *Service) grantTeam(slug, repo string) error {
	_, err := s.GH.Run("api", "--method", "PUT", fmt.Sprintf("orgs/%s/teams/%s/repos/%s/%s", s.C.Organization, slug, s.C.Organization, repo), "-f", "permission="+s.C.GraderPermission)
	return err
}

func (s *Service) grantStudent(repo, login string) error {
	_, err := s.GH.Run("api", "--method", "PUT", fmt.Sprintf("repos/%s/%s/collaborators/%s", s.C.Organization, repo, login), "-f", "permission="+s.C.StudentPermission)
	return err
}

// reconcileStudentRepo checks an existing repository and restores any
// missing student access, grader-team access, or metadata. Repository
// contents are never modified.
func (s *Service) reconcileStudentRepo(repo map[string]any, repoName, slug, name, login string) (string, error) {
	props, err := s.GetRepoProperties(repoName)
	if err != nil {
		return "", err
	}
	if t := props["repo_type"]; t != "" && t != "student" {
		return "", fmt.Errorf("repository %s exists but is not a student repository (repo_type=%s)", repoName, t)
	}
	if owner := props["github_id"]; owner != "" && !strings.EqualFold(owner, login) {
		return "", fmt.Errorf("repository %s already belongs to GitHub user '%s', not '%s'", repoName, owner, login)
	}

	var fixes []func() error
	var todo []string

	collab, err := s.IsCollaborator(repoName, login)
	if err != nil {
		return "", err
	}
	if !collab {
		pending, err := s.hasPendingInvitation(repoName, login)
		if err != nil {
			return "", err
		}
		if !pending {
			todo = append(todo, fmt.Sprintf("add student collaborator %s with: %s", login, s.C.StudentPermission))
			fixes = append(fixes, func() error { return s.grantStudent(repoName, login) })
		}
	}

	_, connected, err := s.TeamRepoPermission(slug, repoName)
	if err != nil {
		return "", err
	}
	if !connected {
		todo = append(todo, fmt.Sprintf("add team '%s' with: %s", s.C.GraderTeam, s.C.GraderPermission))
		fixes = append(fixes, func() error { return s.grantTeam(slug, repoName) })
	}

	missing := map[string]string{}
	for k, v := range map[string]string{"repo_type": "student", "student_name": name, "github_id": login} {
		if props[k] == "" {
			missing[k] = v
		}
	}
	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k+"="+missing[k])
		}
		sort.Strings(keys)
		todo = append(todo, "set custom properties: "+strings.Join(keys, ", "))
		fixes = append(fixes, func() error { return s.SetRepoProperties(repoName, missing) })
	}

	if len(todo) == 0 {
		s.printf("SKIP      %s already exists\n          %s\n", repoName, str(repo["html_url"]))
		return ResultExisting, nil
	}
	if !s.Apply {
		s.printf("DRY RUN   %s already exists; would repair:\n", repoName)
		for _, t := range todo {
			s.printf("          %s\n", t)
		}
		return ResultDryRun, nil
	}
	for _, f := range fixes {
		if err := f(); err != nil {
			return "", err
		}
	}
	s.printf("REPAIRED  %s\n", repoName)
	for _, t := range todo {
		s.printf("          %s\n", t)
	}
	return ResultRepaired, nil
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
	r.FieldsPerRecord = -1 // tolerate ragged rows from spreadsheet exports
	headers, err := r.Read()
	if err != nil {
		return fmt.Errorf("could not read CSV header: %w", err)
	}

	index := make(map[string]int, len(headers))
	for i, h := range headers {
		if i == 0 {
			h = strings.TrimPrefix(h, "\ufeff") // Excel/LMS exports often start with a BOM
		}
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

	s.printf("Processing students from %s\n\n", path)
	counts := map[string]int{}
	row := 1
	for {
		rec, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		row++
		if readErr != nil {
			counts["error"]++
			s.printf("ERROR     row %d: could not read row: %v\n\n", row, readErr)
			continue
		}

		cell := func(column string) string {
			i := index[column]
			if i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		github := cell(githubColumn)
		name := cell(nameColumn)
		repo := ""
		if repoColumn != "" {
			repo = cell(repoColumn)
		}

		if github == "" {
			counts["blank"]++
			s.printf("BLANK     row %d: no GitHub username\n\n", row)
			continue
		}

		result, err := s.CreateStudentRepo(nameOr(name, github), github, repo)
		if err != nil {
			counts["error"]++
			s.printf("ERROR     row %d (%s): %v\n\n", row, nameOr(name, github), err)
			continue
		}
		counts[result]++
		s.println()
	}

	s.println("Batch summary")
	s.printf("  Created:  %d\n", counts[ResultCreated])
	s.printf("  Repaired: %d\n", counts[ResultRepaired])
	s.printf("  Existing: %d\n", counts[ResultExisting])
	s.printf("  Dry run:  %d\n", counts[ResultDryRun])
	s.printf("  Errors:   %d\n", counts["error"])
	s.printf("  Blank:    %d\n", counts["blank"])
	return nil
}

// ---------------------------------------------------------------- student inspection

// findStudentRepo locates a student repository by GitHub ID (custom
// property) or repository name, case-insensitively.
func findStudentRepo(repos []map[string]any, query string) (map[string]any, bool) {
	for _, r := range repos {
		if isStudentRepo(r) && strings.EqualFold(repoProps(r)["github_id"], query) {
			return r, true
		}
	}
	for _, r := range repos {
		if strings.EqualFold(str(r["name"]), query) && str(r["name"]) != "" {
			return r, true
		}
	}
	return nil, false
}

// StudentInfo shows details for a student, looked up by GitHub ID or
// repository name.
func (s *Service) StudentInfo(query string) error {
	repos, err := s.ListRepos()
	if err != nil {
		return err
	}
	repo, found := findStudentRepo(repos, query)

	login := query
	if found {
		if id := repoProps(repo)["github_id"]; id != "" {
			login = id
		}
	}
	u, err := s.UserGet(login)
	if err != nil {
		return err
	}
	login = str(u["login"])

	name := login
	if found && repoProps(repo)["student_name"] != "" {
		name = repoProps(repo)["student_name"]
	} else if n := str(u["name"]); n != "" {
		name = n
	}
	s.printf("Student:      %s\nGitHub:       %s\nGitHub URL:   %s\n", name, login, str(u["html_url"]))
	if !found {
		s.printf("Repository:   NOT CREATED (%s/%s)\n", s.C.Organization, login)
		return nil
	}
	repoName := str(repo["name"])
	s.printf("Repository:   %s\nPrivate:      %s\nDefault:      %s\n", str(repo["html_url"]), str(repo["private"]), str(repo["default_branch"]))
	var perm map[string]any
	if err := s.json(&perm, "api", fmt.Sprintf("repos/%s/%s/collaborators/%s/permission", s.C.Organization, repoName, login)); err == nil && perm != nil {
		s.printf("Student perm: %s\n", permissionFromMap(perm))
	}
	if t, ok, err := s.TeamGet(s.C.GraderTeam); err == nil && ok {
		p, c, _ := s.TeamRepoPermission(str(t["slug"]), repoName)
		if c {
			s.printf("Grader team:  %s (%s)\n", s.C.GraderTeam, p)
		} else {
			s.printf("Grader team:  %s (NOT CONNECTED)\n", s.C.GraderTeam)
		}
	}
	invs, _ := s.PendingInvitations(repoName)
	if len(invs) == 0 {
		s.println("Pending inv.: none")
	} else {
		s.printf("Pending inv.: %d\n", len(invs))
		for _, inv := range invs {
			s.printf("              %s\n", invitee(inv))
		}
	}
	return nil
}

func studentRow(r map[string]any) (name, id, url string) {
	p := repoProps(r)
	id = p["github_id"]
	if id == "" {
		id = str(r["name"])
	}
	return p["student_name"], id, str(r["html_url"])
}

// StudentList lists repositories tagged repo_type=student.
func (s *Service) StudentList() error {
	repos, err := s.ListRepos()
	if err != nil {
		return err
	}
	var students []map[string]any
	for _, r := range repos {
		if isStudentRepo(r) {
			students = append(students, r)
		}
	}
	sort.Slice(students, func(i, j int) bool { return str(students[i]["name"]) < str(students[j]["name"]) })
	s.printf("Student repositories in %s (%d):\n", s.C.Organization, len(students))
	for _, r := range students {
		name, id, url := studentRow(r)
		s.printf("  %-25s %-25s %s\n", name, id, url)
	}
	if other := len(repos) - len(students); other > 0 {
		s.printf("\n(%d other repositories without repo_type=student not shown)\n", other)
	}
	return nil
}

// StudentFind searches student repositories by name, GitHub ID, or repo name.
func (s *Service) StudentFind(q string) error {
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return fmt.Errorf("search query cannot be empty")
	}
	repos, err := s.ListRepos()
	if err != nil {
		return err
	}
	var hits []map[string]any
	for _, r := range repos {
		if !isStudentRepo(r) {
			continue
		}
		p := repoProps(r)
		for _, f := range []string{str(r["name"]), p["student_name"], p["github_id"]} {
			if strings.Contains(strings.ToLower(f), needle) {
				hits = append(hits, r)
				break
			}
		}
	}
	s.printf("Student search: '%s'\n", q)
	if len(hits) == 0 {
		s.println("No matching student repositories found.")
		return nil
	}
	s.printf("Matches (%d):\n", len(hits))
	for _, r := range hits {
		name, id, url := studentRow(r)
		s.printf("  %-25s %-25s %s\n", name, id, url)
	}
	return nil
}

// ---------------------------------------------------------------- doctor

func (s *Service) Doctor(active string) error {
	if _, err := s.GH.Run("auth", "status"); err != nil {
		return err
	}
	var u map[string]any
	if err := s.json(&u, "api", "user"); err != nil {
		return err
	}
	s.printf("mgc doctor\n\nClassroom:       %s\nOrganization:    %s\nAuthenticated:   OK (%s)\n", active, s.C.Organization, str(u["login"]))
	var org map[string]any
	if err := s.json(&org, "api", fmt.Sprintf("orgs/%s", s.C.Organization)); err != nil {
		return err
	}
	s.printf("Organization:    OK (%s)\n", str(org["login"]))
	t, ok, err := s.TeamGet(s.C.GraderTeam)
	if err != nil {
		return err
	}
	if ok {
		s.printf("Grader team:     OK (%s)\nTeam permission: %s\n", str(t["name"]), s.C.GraderPermission)
	} else {
		s.printf("Grader team:     NOT CREATED (%s)\n", s.C.GraderTeam)
	}
	s.printf("Course info URL: %s\n", s.C.CourseInfoURL)
	return nil
}
