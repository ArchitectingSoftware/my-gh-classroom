package cmd

import (
	"reflect"
	"testing"
)

func TestNormalizeArgs(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{[]string{"-cr", "cs281", "student", "list"}, []string{"--classroom", "cs281", "student", "list"}},
		{[]string{"-apply", "team", "create"}, []string{"--apply", "team", "create"}},
		{[]string{"-a", "--classroom", "x"}, []string{"-a", "--classroom", "x"}},
		{[]string{"student", "find", "--", "-cr"}, []string{"student", "find", "--", "-cr"}},
		{nil, []string{}},
	}
	for _, c := range cases {
		got := normalizeArgs(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("normalizeArgs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeArgsDoesNotMutateInput(t *testing.T) {
	in := []string{"-cr", "x"}
	normalizeArgs(in)
	if in[0] != "-cr" {
		t.Error("input slice was modified")
	}
}
