package course

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/canvas"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/gh"
)

// ImportMeta describes an import run for the report header.
type ImportMeta struct {
	File      string
	Classroom string
	Limit     int // 0 means the whole file
	Started   time.Time
}

// ImportSummary tallies the outcome of an import run.
type ImportSummary struct {
	Processed int
	Created   int // in a dry run: would be created
	Skipped   int
	Errors    int
}

type importOutcome struct {
	student canvas.Student
	pos     string // progress label, e.g. "[5/44]"
	status  string // CREATING, WOULD CREATE, SKIPPING, CHECKING
	ok      bool
	detail  string
}

// ImportStudents provisions a repository for each student that does not
// already have one. It is an upsert: students with an existing repository
// (matched by github_id property or repository name) are skipped with no
// changes. Nothing is changed unless s.Apply is set. Progress is written to
// s.Out and, if non-nil, to results.
func (s *Service) ImportStudents(students []canvas.Student, meta ImportMeta, results io.Writer) (ImportSummary, error) {
	w := s.out()
	if results != nil {
		w = io.MultiWriter(w, results)
	}
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }

	mode := "APPLY (changes will be made)"
	if !s.Apply {
		mode = "DRY RUN (no changes will be made; pass -apply to import)"
	}
	records := fmt.Sprintf("%d", len(students))
	if meta.Limit > 0 {
		records += fmt.Sprintf(" (limited by --number %d)", meta.Limit)
	}
	p("Canvas import\n")
	p("  File:         %s\n", meta.File)
	p("  Classroom:    %s (%s)\n", meta.Classroom, s.C.Organization)
	p("  Mode:         %s\n", mode)
	p("  Records:      %s\n", records)
	p("  Started:      %s\n\n", meta.Started.Format("2006-01-02 15:04:05 MST"))

	var sum ImportSummary

	// Preflight: fail once, up front, rather than once per student.
	team, err := s.EnsureTeam(s.C.GraderTeam)
	if err != nil {
		p("ERROR     %v\n          create it with: mgc -apply team create\n", err)
		return sum, err
	}
	slug := str(team["slug"])
	if missing, err := s.missingPropertySchema(); err != nil {
		p("ERROR     could not read custom property schema: %s\n", concise(err))
		return sum, err
	} else if len(missing) > 0 {
		msg := fmt.Sprintf("custom properties not defined in %s: %s; run: mgc -apply properties setup", s.C.Organization, strings.Join(missing, ", "))
		if s.Apply {
			p("ERROR     %s\n", msg)
			return sum, errors.New(msg)
		}
		p("WARNING   %s\n          (an -apply run will stop here until this is fixed)\n\n", msg)
	}
	repos, err := s.ListRepos()
	if err != nil {
		p("ERROR     could not list repositories: %s\n", concise(err))
		return sum, err
	}
	idx := newRepoIndex(repos)

	width := len(fmt.Sprint(len(students)))
	seen := map[string]string{}
	var failures []importOutcome

	for i, st := range students {
		name := st.Name
		if name == "" {
			name = st.GitHubID
		}
		pos := fmt.Sprintf("[%*d/%d]", width, i+1, len(students))
		p("%s %-28s %-22s ", pos, truncate(name, 28), truncate(st.GitHubID, 22))

		o := s.importOne(st, pos, slug, idx, seen, func(status string) { p("%-13s", status) })
		sum.Processed++
		switch {
		case !o.ok:
			sum.Errors++
			failures = append(failures, o)
			p("ERROR    %s\n", o.detail)
		case o.status == "SKIPPING":
			sum.Skipped++
			p("SUCCESS  %s\n", o.detail)
		default:
			sum.Created++
			p("SUCCESS  %s\n", o.detail)
		}
	}

	created := "Created:"
	if !s.Apply {
		created = "Would create:"
	}
	p("\nSummary\n")
	p("  Processed:    %d\n", sum.Processed)
	p("  %-13s %d\n", created, sum.Created)
	p("  Skipped:      %d\n", sum.Skipped)
	p("  Errors:       %d\n", sum.Errors)
	if len(failures) > 0 {
		p("\nErrors\n")
		for _, f := range failures {
			p("  %s %s (GitHub-ID %q, CSV row %d): %s\n", f.pos, f.student.Name, f.student.RawID, f.student.Row, f.detail)
		}
	}
	if !s.Apply {
		p("\nDry run only. Re-run with -apply to make these changes.\n")
	}
	return sum, nil
}

// importOne decides and (if applying) performs the action for one student.
// announce is called with the status as soon as it is known, before any
// mutation, so progress is visible while slow API calls run.
func (s *Service) importOne(st canvas.Student, pos, slug string, idx *repoIndex, seen map[string]string, announce func(string)) importOutcome {
	o := importOutcome{student: st, pos: pos}
	fail := func(status, detail string) importOutcome {
		announce(status)
		o.status, o.detail = status, detail
		return o
	}
	succeed := func(detail string) importOutcome {
		o.ok, o.detail = true, detail
		return o
	}

	id := st.GitHubID
	switch {
	case id == "":
		return fail("CHECKING", "GitHub-ID is blank")
	case !canvas.ValidGitHubID(id):
		return fail("CHECKING", fmt.Sprintf("GitHub-ID %q is not a valid GitHub username", st.RawID))
	}
	key := strings.ToLower(id)
	if first, dup := seen[key]; dup {
		o.status = "SKIPPING"
		announce(o.status)
		return succeed(fmt.Sprintf("duplicate GitHub-ID, already handled at %s", first))
	}
	seen[key] = pos

	if r, ok := idx.forStudent(id); ok {
		if msg := conflict(r, id); msg != "" {
			return fail("CHECKING", msg)
		}
		o.status = "SKIPPING"
		announce(o.status)
		d := "repository exists: " + str(r["html_url"])
		if repoProps(r)["repo_type"] == "" {
			d += fmt.Sprintf(" (missing custom properties; repair with: mgc -apply student create --name %q --github %s --repo %s)", st.Name, id, str(r["name"]))
		}
		return succeed(d)
	}

	u, err := s.UserGet(id)
	if err != nil {
		return fail("CHECKING", concise(err))
	}
	login := str(u["login"])
	note := ""
	if st.Normalized() {
		note = fmt.Sprintf(" (GitHub-ID entered as %q)", st.RawID)
	}

	if !s.Apply {
		o.status = "WOULD CREATE"
		announce(o.status)
		return succeed(fmt.Sprintf("would create %s/%s%s", s.C.Organization, login, note))
	}
	o.status = "CREATING"
	announce(o.status)
	if err := s.createNew(slug, st.Name, login, login); err != nil {
		o.detail = concise(err)
		return o
	}
	idx.add(map[string]any{"name": login, "custom_properties": map[string]any{"repo_type": "student", "github_id": login}})
	return succeed(fmt.Sprintf("https://github.com/%s/%s%s", s.C.Organization, login, note))
}

// missingPropertySchema returns the student custom properties not yet
// defined on the organization.
func (s *Service) missingPropertySchema() ([]string, error) {
	var schema []map[string]any
	if err := s.json(&schema, "api", fmt.Sprintf("orgs/%s/properties/schema", s.C.Organization)); err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, p := range schema {
		have[str(p["property_name"])] = true
	}
	var missing []string
	for _, p := range studentPropertyNames {
		if !have[p.Name] {
			missing = append(missing, p.Name)
		}
	}
	return missing, nil
}

// repoIndex looks up existing repositories by student GitHub ID (custom
// property) and by repository name, case-insensitively.
type repoIndex struct {
	byGitHubID map[string]map[string]any
	byName     map[string]map[string]any
}

func newRepoIndex(repos []map[string]any) *repoIndex {
	idx := &repoIndex{byGitHubID: map[string]map[string]any{}, byName: map[string]map[string]any{}}
	for _, r := range repos {
		idx.add(r)
	}
	return idx
}

func (x *repoIndex) add(r map[string]any) {
	if id := repoProps(r)["github_id"]; id != "" && isStudentRepo(r) {
		x.byGitHubID[strings.ToLower(id)] = r
	}
	if n := str(r["name"]); n != "" {
		x.byName[strings.ToLower(n)] = r
	}
}

func (x *repoIndex) forStudent(id string) (map[string]any, bool) {
	k := strings.ToLower(id)
	if r, ok := x.byGitHubID[k]; ok {
		return r, true
	}
	r, ok := x.byName[k]
	return r, ok
}

// conflict explains why an existing repository named after the student
// cannot be treated as theirs, or returns "" if it can.
func conflict(r map[string]any, id string) string {
	p := repoProps(r)
	if t := p["repo_type"]; t != "" && t != "student" {
		return fmt.Sprintf("repository %s exists but is not a student repository (repo_type=%s)", str(r["name"]), t)
	}
	if owner := p["github_id"]; owner != "" && !strings.EqualFold(owner, id) {
		return fmt.Sprintf("repository %s exists but belongs to GitHub user '%s'", str(r["name"]), owner)
	}
	return ""
}

// concise renders an error on one line, replacing verbose gh command
// dumps (which can include base64 file content) with gh's message.
func concise(err error) string {
	msg := err.Error()
	var ge *gh.Error
	if errors.As(err, &ge) {
		msg = strings.Replace(msg, ge.Error(), ge.Detail, 1)
	}
	return strings.Join(strings.Fields(msg), " ")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "~"
}
