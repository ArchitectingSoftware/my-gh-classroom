package gh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

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
		return nil, fmt.Errorf("gh command failed:\n  gh %s\n\n%s", strings.Join(args, " "), detail)
	}
	return stdout.Bytes(), nil
}

func (c *Client) Run(args ...string) ([]byte, error)                    { return c.run(nil, args...) }
func (c *Client) RunInput(input []byte, args ...string) ([]byte, error) { return c.run(input, args...) }

func (c *Client) JSON(out any, args ...string) error {
	data, err := c.Run(args...)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid JSON from gh: %w", err)
	}
	return nil
}
func (c *Client) JSONInput(out any, input []byte, args ...string) error {
	data, err := c.RunInput(input, args...)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid JSON from gh: %w", err)
	}
	return nil
}
