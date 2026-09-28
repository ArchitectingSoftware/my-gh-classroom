// Package canvas reads student rosters exported from Canvas.
//
// The expected export is a Canvas quiz/survey "Student Analysis" CSV in
// which students entered their GitHub username. Only two columns are used:
// "Name" (the student's display name, e.g. "Jane Smith") and "GitHub-ID".
// All other columns are ignored.
package canvas

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	NameColumn   = "Name"
	GitHubColumn = "GitHub-ID"
)

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
func ReadFile(path string, limit int) ([]Student, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", path, err)
	}
	defer f.Close()
	return Read(f, limit)
}

// Read parses a Canvas export. It validates the header first and returns
// an error wrapping ErrMissingColumns if "Name" or "GitHub-ID" is absent.
// If limit > 0, at most limit data rows are returned.
func Read(r io.Reader, limit int) ([]Student, error) {
	if limit < 0 {
		return nil, fmt.Errorf("record limit must be zero (all) or positive, got %d", limit)
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
	for _, c := range []string{NameColumn, GitHubColumn} {
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
		raw := cell(rec, GitHubColumn)
		out = append(out, Student{
			Row:      line,
			Name:     strings.Join(strings.Fields(cell(rec, NameColumn)), " "),
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
