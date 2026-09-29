package course

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/gh"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/roster"
)

// ImportMeta describes an import run.
type ImportMeta struct {
	File         string
	Classroom    string
	Limit        int // 0 means the whole file
	NameColumn   string
	GitHubColumn string
	Repair       bool // repair existing repositories instead of skipping them
	Started      time.Time
}

// ImportSummary tallies the outcome of an import run. In a dry run,
// Created and Repaired count what would be created or repaired.
type ImportSummary struct {
	Processed int
	Created   int
	Repaired  int
	Skipped   int
	Errors    int
	// Messages asks students to fix problems only they can fix (a blank,
	// malformed, or nonexistent GitHub username).
	Messages []Message
}

// Per-student statuses shown in the report.
const (
	statusChecking    = "CHECKING"
	statusCreating    = "CREATING"
	statusWouldCreate = "WOULD CREATE"
	statusRepairing   = "REPAIRING"
	statusWouldRepair = "WOULD REPAIR"
	statusSkipping    = "SKIPPING"
)

type importOutcome struct {
	student roster.Student
	pos     string // progress label, e.g. "[5/44]"
	status  string
	ok      bool
	detail  string
	problem string // student-fixable problem, for --message
}

// ImportStudents provisions a repository for each student that does not
// already have one. It is an upsert: students with an existing repository
// (matched by github_id property or by repository name) are skipped with
// no changes, unless meta.Repair is set, in which case anything missing
// from their repository is repaired. Nothing is changed unless s.Apply is
// set. Progress is written to s.Out and, if non-nil, to results.
func (s *Service) ImportStudents(students []roster.Student, meta ImportMeta, results io.Writer) (ImportSummary, error) {
	w := s.out()
	if results != nil {
		w = io.MultiWriter(w, results)
	}
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }

	mode := "APPLY (changes will be made)"
	if !s.Apply {
		mode = "DRY RUN (no changes will be made; pass --apply to import)"
	}
	records := fmt.Sprintf("%d", len(students))
	if meta.Limit > 0 {
		records += fmt.Sprintf(" (limited by --number %d)", meta.Limit)
	}
	existing := "skip (use --repair to fix incomplete repositories)"
	if meta.Repair {
		existing = "repair anything missing (--repair)"
	}
	p("Roster import\n")
	p("  File:         %s\n", meta.File)
	if meta.NameColumn != "" || meta.GitHubColumn != "" {
		p("  Columns:      name %q, GitHub ID %q\n", meta.NameColumn, meta.GitHubColumn)
	}
	p("  Classroom:    %s (%s)\n", meta.Classroom, s.C.Organization)
	p("  Repo names:   %s<github-id>\n", s.C.RepoPrefix)
	p("  Existing:     %s\n", existing)
	p("  Mode:         %s\n", mode)
	p("  Records:      %s\n", records)
	p("  Started:      %s\n\n", meta.Started.Format("2006-01-02 15:04:05 MST"))

	var sum ImportSummary

	// Preflight: fail once, up front, rather than once per student.
	team, err := s.EnsureTeam(s.C.GraderTeam)
	if err != nil {
		p("ERROR     %v\n          create it with: mgc --apply team create\n", err)
		return sum, err
	}
	slug := str(team["slug"])
	if missing, err := s.missingPropertySchema(); err != nil {
		p("ERROR     could not read custom property schema: %s\n", concise(err))
		return sum, err
	} else if len(missing) > 0 {
		msg := fmt.Sprintf("custom properties not defined in %s: %s; run: mgc --apply properties setup", s.C.Organization, strings.Join(missing, ", "))
		if s.Apply {
			p("ERROR     %s\n", msg)
			return sum, errors.New(msg)
		}
		p("WARNING   %s\n          (an --apply run will stop here until this is fixed)\n\n", msg)
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

		o := s.importOne(st, pos, slug, meta.Repair, idx, seen, func(status string) { p("%-13s", status) })
		sum.Processed++
		if !o.ok {
			sum.Errors++
			failures = append(failures, o)
			if m, ok := s.rosterProblemMessage(st, o.problem); ok {
				sum.Messages = append(sum.Messages, m)
			}
			p("ERROR    %s\n", o.detail)
			continue
		}
		switch o.status {
		case statusSkipping:
			sum.Skipped++
		case statusRepairing, statusWouldRepair:
			sum.Repaired++
		default:
			sum.Created++
		}
		p("SUCCESS  %s\n", o.detail)
	}

	created, repaired := "Created:", "Repaired:"
	if !s.Apply {
		created, repaired = "Would create:", "Would repair:"
	}
	p("\nSummary\n")
	p("  Processed:    %d\n", sum.Processed)
	p("  %-13s %d\n", created, sum.Created)
	if meta.Repair {
		p("  %-13s %d\n", repaired, sum.Repaired)
	}
	p("  Skipped:      %d\n", sum.Skipped)
	p("  Errors:       %d\n", sum.Errors)
	if len(failures) > 0 {
		p("\nErrors\n")
		for _, f := range failures {
			p("  %s %s (GitHub ID %q, CSV row %d): %s\n", f.pos, f.student.Name, f.student.RawID, f.student.Row, f.detail)
		}
	}
	if !s.Apply {
		p("\nDry run only. Re-run with --apply to make these changes.\n")
	}
	return sum, nil
}

// importOne decides and (if applying) performs the action for one student.
// announce is called with the status as soon as it is known, before any
// mutation, so progress is visible while slow API calls run.
func (s *Service) importOne(st roster.Student, pos, slug string, repair bool, idx *repoIndex, seen map[string]string, announce func(string)) importOutcome {
	o := importOutcome{student: st, pos: pos}
	begin := func(status string) {
		o.status = status
		announce(status)
	}
	fail := func(status, detail string) importOutcome {
		begin(status)
		o.detail = detail
		return o
	}
	succeed := func(detail string) importOutcome {
		o.ok, o.detail = true, detail
		return o
	}

	problem := func(kind string, out importOutcome) importOutcome {
		out.problem = kind
		return out
	}
	id := st.GitHubID
	switch {
	case id == "":
		return problem(problemBlank, fail(statusChecking, "GitHub ID is blank"))
	case !roster.ValidGitHubID(id):
		return problem(problemInvalid, fail(statusChecking, fmt.Sprintf("GitHub ID %q is not a valid GitHub username", st.RawID)))
	}
	key := strings.ToLower(id)
	if first, dup := seen[key]; dup {
		begin(statusSkipping)
		return succeed(fmt.Sprintf("duplicate GitHub ID, already handled at %s", first))
	}
	seen[key] = pos

	if r, ok := idx.forStudent(id, s.C.RepoPrefix); ok {
		if msg := conflict(r, id); msg != "" {
			return fail(statusChecking, msg)
		}
		if !repair {
			begin(statusSkipping)
			d := "repository exists: " + str(r["html_url"])
			if repoProps(r)["repo_type"] == "" {
				d += " (missing custom properties; re-run with --repair to fix)"
			}
			return succeed(d)
		}
		return s.repairExisting(r, st, slug, begin, succeed, fail)
	}

	u, err := s.UserGet(id)
	var nf *UserNotFoundError
	if errors.As(err, &nf) {
		return problem(problemNotFound, fail(statusChecking, concise(err)))
	}
	if err != nil {
		return fail(statusChecking, concise(err))
	}
	login := str(u["login"])
	repoName := s.RepoName(login)
	if err := ValidateRepoName(repoName); err != nil {
		return fail(statusChecking, err.Error())
	}
	note := ""
	if st.Normalized() {
		note = fmt.Sprintf(" (GitHub ID entered as %q)", st.RawID)
	}

	if !s.Apply {
		begin(statusWouldCreate)
		return succeed(fmt.Sprintf("would create %s/%s%s", s.C.Organization, repoName, note))
	}
	begin(statusCreating)
	if err := s.createNew(slug, st.Name, login, repoName); err != nil {
		var pe *PartialError
		if errors.As(err, &pe) {
			o.detail = fmt.Sprintf("repository %s was created but setup did not finish (%s); re-run with --repair to finish it", repoName, concise(pe.Err))
		} else {
			o.detail = concise(err)
		}
		return o
	}
	idx.add(map[string]any{"name": repoName, "custom_properties": map[string]any{"repo_type": "student", "github_id": login}})
	return succeed(fmt.Sprintf("https://github.com/%s/%s%s", s.C.Organization, repoName, note))
}

// repairExisting plans and (if applying) performs repairs on a student's
// existing repository.
func (s *Service) repairExisting(r map[string]any, st roster.Student, slug string, begin func(string), succeed func(string) importOutcome, fail func(string, string) importOutcome) importOutcome {
	repoName := str(r["name"])
	login := repoProps(r)["github_id"]
	if login == "" {
		login = st.GitHubID
	}
	plan, err := s.planRepair(repoName, slug, st.Name, login)
	if err != nil {
		return fail(statusChecking, concise(err))
	}
	if !plan.Needed() {
		begin(statusSkipping)
		return succeed("repository exists, nothing to repair: " + str(r["html_url"]))
	}
	if !s.Apply {
		begin(statusWouldRepair)
		return succeed(fmt.Sprintf("%s: would %s", repoName, strings.Join(plan.Todo, "; ")))
	}
	begin(statusRepairing)
	if err := plan.Apply(); err != nil {
		o := succeed(fmt.Sprintf("%s: repair failed: %s", repoName, concise(err))) // status already announced
		o.ok = false
		return o
	}
	return succeed(fmt.Sprintf("%s: %s", repoName, strings.Join(plan.Todo, "; ")))
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

// forStudent finds a student's repository by github_id property, then by
// the prefixed repository name, then by the bare GitHub ID (e.g. a repo
// created before a prefix was configured).
func (x *repoIndex) forStudent(id, prefix string) (map[string]any, bool) {
	k := strings.ToLower(id)
	if r, ok := x.byGitHubID[k]; ok {
		return r, true
	}
	for _, name := range []string{strings.ToLower(prefix) + k, k} {
		if r, ok := x.byName[name]; ok {
			return r, true
		}
	}
	return nil, false
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
