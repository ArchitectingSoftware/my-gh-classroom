package gh

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("HTTP 404"), false}, // only gh.Error counts
		{&Error{Detail: "gh: Not Found (HTTP 404)"}, true},
		{fmt.Errorf("wrapped: %w", &Error{Detail: "gh: Not Found (HTTP 404)"}), true},
		{&Error{Detail: "gh: Resource not accessible (HTTP 403)"}, false},
		{&Error{Detail: "repo named 404-lab failed (HTTP 500)"}, false},
	}
	for _, c := range cases {
		if got := IsNotFound(c.err); got != c.want {
			t.Errorf("IsNotFound(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

type stub struct {
	out []byte
	err error
}

func (s stub) Run(...string) ([]byte, error)              { return s.out, s.err }
func (s stub) RunInput([]byte, ...string) ([]byte, error) { return s.out, s.err }

func TestJSON(t *testing.T) {
	var v map[string]any
	if err := JSON(stub{out: []byte(`{"login":"x"}`)}, &v, "api", "user"); err != nil || v["login"] != "x" {
		t.Errorf("got %v, %v", v, err)
	}
	var empty map[string]any
	if err := JSON(stub{out: []byte("  \n")}, &empty); err != nil || empty != nil {
		t.Errorf("empty body: %v, %v", empty, err)
	}
	if err := JSON(stub{out: []byte("not json")}, &v); err == nil {
		t.Error("expected decode error")
	}
	if err := JSON(stub{err: errors.New("boom")}, &v); err == nil {
		t.Error("expected run error")
	}
}
