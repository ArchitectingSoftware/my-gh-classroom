package course

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/config"
	"github.com/ArchitectingSoftware/my-gh-classroom/internal/gh"
)

// call is one recorded `gh api` invocation.
type call struct {
	Method string
	Path   string
	Fields []string // -f / -F values
	Input  []byte
	Args   []string
}

type reply struct {
	body string
	err  error
}

// fakeGH routes `gh api` invocations by "METHOD path" and records them.
type fakeGH struct {
	t      *testing.T
	routes map[string]reply
	calls  []call
}

func newFake(t *testing.T) *fakeGH {
	t.Helper()
	return &fakeGH{t: t, routes: map[string]reply{}}
}

var errNotFound = &gh.Error{Detail: "gh: Not Found (HTTP 404)"}

func (f *fakeGH) on(method, path, body string) *fakeGH {
	f.routes[method+" "+path] = reply{body: body}
	return f
}

func (f *fakeGH) fail(method, path string, err error) *fakeGH {
	f.routes[method+" "+path] = reply{err: err}
	return f
}

func parse(args []string, input []byte) call {
	c := call{Method: "GET", Input: input, Args: args}
	if len(args) > 0 && args[0] != "api" {
		c.Path = strings.Join(args, " ")
		return c
	}
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--method":
			i++
			c.Method = args[i]
		case "-H", "--input":
			i++
		case "-f", "-F":
			i++
			c.Fields = append(c.Fields, args[i])
		case "--paginate":
		default:
			c.Path = a
		}
	}
	return c
}

func (f *fakeGH) handle(args []string, input []byte) ([]byte, error) {
	c := parse(args, input)
	f.calls = append(f.calls, c)
	key := c.Method + " " + c.Path
	r, ok := f.routes[key]
	if !ok {
		f.t.Errorf("unexpected gh call: %s", key)
		return nil, fmt.Errorf("unexpected gh call: %s", key)
	}
	return []byte(r.body), r.err
}

func (f *fakeGH) Run(args ...string) ([]byte, error) { return f.handle(args, nil) }
func (f *fakeGH) RunInput(input []byte, args ...string) ([]byte, error) {
	return f.handle(args, input)
}

// mutations returns every non-GET call as "METHOD path".
func (f *fakeGH) mutations() []string {
	var out []string
	for _, c := range f.calls {
		if c.Method != "GET" {
			out = append(out, c.Method+" "+c.Path)
		}
	}
	return out
}

func (f *fakeGH) find(method, path string) (call, bool) {
	for _, c := range f.calls {
		if c.Method == method && c.Path == path {
			return c, true
		}
	}
	return call{}, false
}

const org = "CS281-Arch-FA26"

func testClassroom() config.Classroom {
	return config.Classroom{
		Organization:      org,
		GraderTeam:        "graders",
		GraderPermission:  "push",
		StudentPermission: "push",
		CourseInfoRepo:    "course-info",
		CourseInfoURL:     "https://github.com/" + org + "/course-info",
		CourseName:        "CS281",
	}
}

func newService(t *testing.T, apply bool) (*Service, *fakeGH, *bytes.Buffer) {
	t.Helper()
	f := newFake(t)
	var out bytes.Buffer
	return &Service{GH: f, C: testClassroom(), Apply: apply, Out: &out}, f, &out
}

// Common route helpers.

func (f *fakeGH) team() *fakeGH {
	return f.on("GET", "orgs/"+org+"/teams/graders", `{"name":"graders","slug":"graders"}`)
}

func (f *fakeGH) user(login string) *fakeGH {
	return f.on("GET", "users/"+login, fmt.Sprintf(`{"login":%q,"html_url":"https://github.com/%s"}`, login, login))
}

func (f *fakeGH) repos(body string) *fakeGH {
	return f.on("GET", "orgs/"+org+"/repos?per_page=100&page=1&type=all&sort=full_name&direction=asc", body)
}

func repoPath(repo, rest string) string {
	p := "repos/" + org + "/" + repo
	if rest != "" {
		p += "/" + rest
	}
	return p
}

func teamRepoPath(repo string) string {
	return "orgs/" + org + "/teams/graders/repos/" + org + "/" + repo
}
