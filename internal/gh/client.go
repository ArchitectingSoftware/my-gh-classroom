package gh

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// API is the subset of GitHub CLI behavior the rest of mgc depends on.
// The production implementation shells out to `gh`; tests substitute a fake.
type API interface {
	Run(args ...string) ([]byte, error)
	RunInput(input []byte, args ...string) ([]byte, error)
}

// Error is returned when a gh invocation exits non-zero.
type Error struct {
	Args   []string
	Detail string
}

func (e *Error) Error() string {
	return fmt.Sprintf("gh command failed:\n  gh %s\n\n%s", strings.Join(e.Args, " "), e.Detail)
}

// IsNotFound reports whether err is a gh API failure caused by an HTTP 404.
func IsNotFound(err error) bool { return IsStatus(err, 404) }

// IsStatus reports whether err is a gh API failure with the given HTTP
// status, as reported by gh (e.g. "gh: Not Found (HTTP 404)").
func IsStatus(err error, code int) bool {
	var ge *Error
	if !errors.As(err, &ge) {
		return false
	}
	return strings.Contains(ge.Detail, fmt.Sprintf("(HTTP %d)", code))
}

type Client struct{}

func New() *Client { return &Client{} }

func (c *Client) run(stdin []byte, args ...string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("GitHub CLI 'gh' was not found. Install it with: brew install gh")
	}
	cmd := exec.Command("gh", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return nil, &Error{Args: args, Detail: detail}
	}
	return stdout.Bytes(), nil
}

func (c *Client) Run(args ...string) ([]byte, error)                    { return c.run(nil, args...) }
func (c *Client) RunInput(input []byte, args ...string) ([]byte, error) { return c.run(input, args...) }

// JSON runs a gh command and decodes its stdout into out.
func JSON(api API, out any, args ...string) error {
	data, err := api.Run(args...)
	if err != nil {
		return err
	}
	return decode(data, out)
}

// JSONInput runs a gh command with stdin and decodes its stdout into out.
func JSONInput(api API, out any, input []byte, args ...string) error {
	data, err := api.RunInput(input, args...)
	if err != nil {
		return err
	}
	return decode(data, out)
}

func decode(data []byte, out any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid JSON from gh: %w", err)
	}
	return nil
}
