package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	DefaultGraderPermission  = "push"
	DefaultStudentPermission = "push"
	DefaultCourseInfoRepo    = "course-info"

	// RepoNameCaseLower lowercases student repository names (the default).
	RepoNameCaseLower = "lower"
	// RepoNameCasePreserve keeps the case of repo_prefix and the student's
	// GitHub login as GitHub reports it.
	RepoNameCasePreserve = "preserve"
)

type Classroom struct {
	Organization      string `json:"organization"`
	GraderTeam        string `json:"grader_team"`
	GraderPermission  string `json:"grader_permission"`
	StudentPermission string `json:"student_permission"`
	CourseInfoRepo    string `json:"course_info_repo"`
	CourseInfoURL     string `json:"course_info_url"`
	// CourseName is the human-readable course label used in generated
	// repository descriptions and READMEs. Optional; defaults to the
	// upper-cased alias. Always written (even when empty) so the setting
	// is visible in config.json.
	CourseName string `json:"course_name"`
	// RepoPrefix is prepended to the student's GitHub ID to form their
	// repository name, e.g. "cs472" gives "cs472-jsmith42" (a dash is added
	// unless the prefix already ends in '-', '_' or '.'). Optional;
	// defaults to "" (the repository is named after the GitHub ID).
	// Always written (even when empty) so the setting is visible.
	RepoPrefix string `json:"repo_prefix"`
	// RepoNameCase controls the case of generated student repository
	// names: "lower" (default) gives "cs472-jsmith42"; "preserve" keeps
	// the prefix and GitHub login as-is, e.g. "CS472-JSmith42". GitHub
	// treats repository names case-insensitively, so this only affects
	// how names look. Always written so the setting is visible.
	RepoNameCase string `json:"repo_name_case"`
	// Instructors sign messages generated with --message, e.g.
	// ["Dr. Brian Mitchell"]. Optional; when empty, messages are signed
	// "The <course_name> teaching team". Always written (as [] when empty).
	Instructors []string `json:"instructors"`
}

type Config struct {
	DefaultClassroom string               `json:"default_classroom"`
	Classrooms       map[string]Classroom `json:"classrooms"`
}

var repoNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var classroomAliasRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var repoPrefixRE = regexp.MustCompile(`^[A-Za-z0-9._-]{0,60}$`)

// ResolvePath determines which config file to use. Precedence:
//  1. explicit path (the --config flag)
//  2. $MGC_CONFIG
//  3. ./config.json, if it exists
//  4. ~/.mgc/config.json, if it exists
//  5. <user config dir>/mgc/config.json, if it exists (e.g.
//     ~/Library/Application Support/mgc/config.json on macOS,
//     ~/.config/mgc/config.json on Linux)
//
// When none exists it returns ~/.mgc/config.json, the recommended
// location, so error messages point there.
func ResolvePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("MGC_CONFIG"); env != "" {
		return env
	}
	if exists(LocalPath) {
		return LocalPath
	}
	home := HomePath()
	if home != "" && exists(home) {
		return home
	}
	if dir, err := os.UserConfigDir(); err == nil {
		if p := filepath.Join(dir, "mgc", "config.json"); exists(p) {
			return p
		}
	}
	if home != "" {
		return home
	}
	return LocalPath
}

// LocalPath is the per-directory config file, highest precedence after
// --config and $MGC_CONFIG.
const LocalPath = "config.json"

// HomePath is ~/.mgc/config.json, or "" if the home directory is unknown.
func HomePath() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ""
	}
	return filepath.Join(h, ".mgc", "config.json")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func Load(path string) (Config, error) {
	path = ResolvePath(path)
	abs, err := filepath.Abs(path)
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("config file not found: %s\ncreate one with 'mgc init' (./config.json) or 'mgc init --home' (~/.mgc/config.json), or point to one with --config or $MGC_CONFIG", abs)
		}
		return Config{}, fmt.Errorf("could not read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("could not parse config: %w", err)
	}
	if c.Classrooms == nil {
		c.Classrooms = map[string]Classroom{}
	}
	if err := ValidateConfig(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Encode renders a config as it is written to disk.
func Encode(c Config) ([]byte, error) {
	// Write optional settings explicitly (lists as [] rather than null,
	// repo_name_case as its default) so they stay visible and easy to
	// change. Work on a copy so the caller's config is not modified.
	classrooms := make(map[string]Classroom, len(c.Classrooms))
	for alias, cl := range c.Classrooms {
		if cl.Instructors == nil {
			cl.Instructors = []string{}
		}
		if cl.RepoNameCase == "" {
			cl.RepoNameCase = RepoNameCaseLower
		}
		classrooms[alias] = cl
	}
	c.Classrooms = classrooms
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("could not encode config: %w", err)
	}
	return append(data, '\n'), nil
}

func Save(path string, c Config) error {
	path = ResolvePath(path)
	data, err := Encode(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}
	return nil
}

// ErrExists is returned by Create when the file is already there.
var ErrExists = errors.New("config file already exists")

// Create writes a new config file at path, creating its directory. It
// never overwrites: if the file exists it returns ErrExists.
func Create(path string, c Config) error {
	data, err := Encode(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrExists
		}
		return fmt.Errorf("could not write config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("could not write config: %w", err)
	}
	return f.Close()
}

// NewClassroom returns a classroom with obvious placeholder values for
// the user to replace.
func NewClassroom(alias string) Classroom {
	return Classroom{
		Organization:      "YOUR_GITHUB_ORGANIZATION",
		GraderTeam:        "graders",
		GraderPermission:  DefaultGraderPermission,
		StudentPermission: DefaultStudentPermission,
		CourseInfoRepo:    "YOUR_COURSE_INFO_REPO",
		CourseInfoURL:     "https://github.com/YOUR_GITHUB_ORGANIZATION/YOUR_COURSE_INFO_REPO",
		CourseName:        strings.ToUpper(alias),
		RepoPrefix:        "",
		RepoNameCase:      RepoNameCaseLower,
		Instructors:       []string{},
	}
}

// ExampleAlias is the classroom alias used when mgc init is given none;
// config.example.json is Scaffold(ExampleAlias).
const ExampleAlias = "cs101"

// Scaffold returns a starter config with one placeholder classroom,
// which is also the default classroom.
func Scaffold(alias string) Config {
	return Config{
		DefaultClassroom: alias,
		Classrooms:       map[string]Classroom{alias: NewClassroom(alias)},
	}
}

func (c Config) Classroom(alias string) (Classroom, error) {
	if alias == "" {
		alias = c.DefaultClassroom
	}
	if alias == "" {
		return Classroom{}, fmt.Errorf("no classroom selected and no default classroom is configured; use --classroom/-c or set a default")
	}
	cl, ok := c.Classrooms[alias]
	if !ok {
		return Classroom{}, fmt.Errorf("classroom '%s' does not exist", alias)
	}
	if err := ValidateClassroom(alias, cl); err != nil {
		return Classroom{}, err
	}
	return applyDefaults(cl), nil
}

func (c Config) ClassroomAliases() []string {
	out := make([]string, 0, len(c.Classrooms))
	for alias := range c.Classrooms {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

// ClassroomWithAlias is like Classroom but also fills CourseName from the
// alias when it is not configured.
func (c Config) ClassroomWithAlias(alias string) (string, Classroom, error) {
	if alias == "" {
		alias = c.DefaultClassroom
	}
	cl, err := c.Classroom(alias)
	if err != nil {
		return "", Classroom{}, err
	}
	if strings.TrimSpace(cl.CourseName) == "" {
		cl.CourseName = strings.ToUpper(alias)
	}
	return alias, cl, nil
}

func applyDefaults(c Classroom) Classroom {
	if c.GraderPermission == "" {
		c.GraderPermission = DefaultGraderPermission
	}
	if c.StudentPermission == "" {
		c.StudentPermission = DefaultStudentPermission
	}
	if c.CourseInfoRepo == "" {
		c.CourseInfoRepo = DefaultCourseInfoRepo
	}
	if c.RepoNameCase == "" {
		c.RepoNameCase = RepoNameCaseLower
	}
	return c
}

// NormalizeRepoPrefix returns the prefix actually used in repository
// names: a "-" separator is added unless the prefix is empty or already
// ends in '-', '_' or '.'. So "cs472" and "cs472-" both give "cs472-".
func NormalizeRepoPrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || strings.ContainsAny(p[len(p)-1:], "-_.") {
		return p
	}
	return p + "-"
}

func ValidateClassroom(alias string, c Classroom) error {
	if !classroomAliasRE.MatchString(alias) {
		return fmt.Errorf("invalid classroom alias '%s'; use letters, numbers, '.', '_', or '-' and do not start with punctuation", alias)
	}
	c = applyDefaults(c)
	for key, value := range map[string]string{
		"organization":     c.Organization,
		"grader_team":      c.GraderTeam,
		"course_info_repo": c.CourseInfoRepo,
		"course_info_url":  c.CourseInfoURL,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("classroom '%s' must contain a non-empty %q", alias, key)
		}
	}
	if !repoNameRE.MatchString(c.CourseInfoRepo) {
		return fmt.Errorf("classroom '%s' has invalid course_info_repo '%s'", alias, c.CourseInfoRepo)
	}
	if !repoPrefixRE.MatchString(c.RepoPrefix) {
		return fmt.Errorf("classroom '%s' has invalid repo_prefix '%s'; use up to 60 letters, digits, '.', '_', or '-'", alias, c.RepoPrefix)
	}
	if c.RepoNameCase != RepoNameCaseLower && c.RepoNameCase != RepoNameCasePreserve {
		return fmt.Errorf("classroom '%s' has invalid repo_name_case '%s'; use \"lower\" or \"preserve\"", alias, c.RepoNameCase)
	}
	valid := map[string]bool{"pull": true, "triage": true, "push": true, "maintain": true, "admin": true}
	if !valid[c.GraderPermission] || !valid[c.StudentPermission] {
		return fmt.Errorf("classroom '%s' permissions must be pull, triage, push, maintain, or admin", alias)
	}
	u, err := url.Parse(c.CourseInfoURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("classroom '%s' has invalid course_info_url '%s'", alias, c.CourseInfoURL)
	}
	return nil
}

func ValidateConfig(c Config) error {
	for alias, cl := range c.Classrooms {
		if err := ValidateClassroom(alias, cl); err != nil {
			return err
		}
	}
	if c.DefaultClassroom != "" {
		if _, ok := c.Classrooms[c.DefaultClassroom]; !ok {
			return fmt.Errorf("default classroom '%s' does not exist", c.DefaultClassroom)
		}
	}
	return nil
}
