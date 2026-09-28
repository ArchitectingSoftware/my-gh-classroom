package config

import (
	"encoding/json"
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
)

type Classroom struct {
	Organization      string `json:"organization"`
	GraderTeam        string `json:"grader_team"`
	GraderPermission  string `json:"grader_permission"`
	StudentPermission string `json:"student_permission"`
	CourseInfoRepo    string `json:"course_info_repo"`
	CourseInfoURL     string `json:"course_info_url"`
	// CourseName is the human-readable course label used in generated
	// repository descriptions and READMEs. Optional; defaults to the alias.
	CourseName string `json:"course_name,omitempty"`
}

type Config struct {
	DefaultClassroom string               `json:"default_classroom"`
	Classrooms       map[string]Classroom `json:"classrooms"`
}

var repoNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var classroomAliasRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ResolvePath determines which config file to use. Precedence:
//  1. explicit path (the --config flag)
//  2. $MGC_CONFIG
//  3. ./config.json, if it exists
//  4. <user config dir>/mgc/config.json (e.g. ~/.config/mgc/config.json on
//     Linux, ~/Library/Application Support/mgc/config.json on macOS)
func ResolvePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("MGC_CONFIG"); env != "" {
		return env
	}
	if _, err := os.Stat("config.json"); err == nil {
		return "config.json"
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "mgc", "config.json")
	}
	return "config.json"
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
			return Config{}, fmt.Errorf("config file not found: %s (use --config, set MGC_CONFIG, or create ./config.json)", abs)
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

func Save(path string, c Config) error {
	path = ResolvePath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}
	return nil
}

func (c Config) Classroom(alias string) (Classroom, error) {
	if alias == "" {
		alias = c.DefaultClassroom
	}
	if alias == "" {
		return Classroom{}, fmt.Errorf("no classroom selected and no default classroom is configured; use --classroom/-cr or set a default")
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
	return c
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
