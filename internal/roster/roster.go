// Package roster reads student rosters from CSV files.
//
// Any CSV works as long as its header row contains a student-name column
// and a GitHub-username column, by default "Name" and "GitHub-ID" (the
// names can be overridden). All other columns are ignored, so exports
// from an LMS survey (e.g. a Canvas "Student Analysis" report), a
// spreadsheet, or a hand-written file can be used unchanged.
package roster

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Default column names.
const (
	NameColumn   = "Name"
	GitHubColumn = "GitHub-ID"
)

// Options controls how a roster is read. The zero value reads every row
// using the default column names.
type Options struct {
	Limit        int    // if > 0, read at most this many data rows
	NameColumn   string // default "Name"
	GitHubColumn string // default "GitHub-ID"
}

func (o Options) withDefaults() Options {
	if strings.TrimSpace(o.NameColumn) == "" {
		o.NameColumn = NameColumn
	}
	if strings.TrimSpace(o.GitHubColumn) == "" {
		o.GitHubColumn = GitHubColumn
	}
	o.NameColumn, o.GitHubColumn = strings.TrimSpace(o.NameColumn), strings.TrimSpace(o.GitHubColumn)
	return o
}

// Student is one data row from the export.
type Student struct {
	Row      int    // 1-based line number in the CSV (header is row 1)
	Name     string // whitespace-normalized display name
	GitHubID string // normalized GitHub username; empty if the cell was blank
	RawID    string // GitHub-ID cell exactly as entered (trimmed)
}

// Normalized reports whether GitHubID differs from what the student typed,
// for example because a leading "@" or a github.com URL was stripped.
func (s Student) Normalized() bool { return s.RawID != "" && s.RawID != s.GitHubID }

// ErrMissingColumns is returned (wrapped) when required columns are absent.
var ErrMissingColumns = errors.New("missing required columns")

// ReadFile opens path and parses it with Read.
func ReadFile(path string, opts Options) ([]Student, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", path, err)
	}
	defer f.Close()
	return Read(f, opts)
}

// Read parses a roster CSV. It validates the header first and returns
// an error wrapping ErrMissingColumns if the name or GitHub column is
// absent. Column names are matched exactly, ignoring surrounding spaces.
func Read(r io.Reader, opts Options) ([]Student, error) {
	opts = opts.withDefaults()
	limit := opts.Limit
	if limit < 0 {
		return nil, fmt.Errorf("record limit must be zero (all) or positive, got %d", limit)
	}
	if opts.NameColumn == opts.GitHubColumn {
		return nil, fmt.Errorf("name and GitHub columns must differ (both are %q)", opts.NameColumn)
	}
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("file is empty")
	}
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	index := map[string]int{}
	for i, h := range header {
		if i == 0 {
			h = strings.TrimPrefix(h, "\ufeff")
		}
		index[strings.TrimSpace(h)] = i
	}
	var missing []string
	for _, c := range []string{opts.NameColumn, opts.GitHubColumn} {
		if _, ok := index[c]; !ok {
			missing = append(missing, fmt.Sprintf("%q", c))
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w %s; found: %s", ErrMissingColumns, strings.Join(missing, ", "), strings.Join(header, ", "))
	}

	cell := func(rec []string, col string) string {
		i := index[col]
		if i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	var out []Student
	for limit == 0 || len(out) < limit {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line, _ := cr.FieldPos(0)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", line, err)
		}
		raw := cell(rec, opts.GitHubColumn)
		out = append(out, Student{
			Row:      line,
			Name:     strings.Join(strings.Fields(cell(rec, opts.NameColumn)), " "),
			GitHubID: NormalizeGitHubID(raw),
			RawID:    raw,
		})
	}
	return out, nil
}

var urlPrefixRE = regexp.MustCompile(`(?i)^(https?://)?(www\.)?github\.com/`)

// NormalizeGitHubID cleans common ways students mistype a username:
// surrounding whitespace, a leading "@", or a full profile URL.
func NormalizeGitHubID(s string) string {
	s = strings.TrimSpace(s)
	s = urlPrefixRE.ReplaceAllString(s, "")
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimSuffix(s, "/")
	return strings.TrimSpace(s)
}

// usernameRE is deliberately permissive (GitHub's rules have changed over
// time); the account's existence is verified against the API anyway.
var usernameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// ValidGitHubID reports whether s looks like a GitHub username.
func ValidGitHubID(s string) bool { return usernameRE.MatchString(s) }
