package cmd

import (
	"strings"
	"testing"
)

func TestRejectLegacyArgs(t *testing.T) {
	cases := []struct {
		args []string
		want string // substring of the error; "" means no error
	}{
		{[]string{"-apply", "team", "create"}, "use --apply or -a"},
		{[]string{"-cr", "cs281", "student", "list"}, "use --classroom or -c"},
		{[]string{"-cr=cs281", "student", "list"}, "use --classroom or -c"},
		{[]string{"-a", "-c", "cs281", "team", "create"}, ""},
		{[]string{"--apply", "--classroom", "cs281", "team", "create"}, ""},
		{[]string{"student", "find", "--", "-cr"}, ""}, // after "--" is data
		{nil, ""},
	}
	for _, c := range cases {
		err := rejectLegacyArgs(c.args)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%v: unexpected error %v", c.args, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%v: err = %v, want %q", c.args, err, c.want)
		}
	}
}
