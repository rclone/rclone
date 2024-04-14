package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestsToRegexp(t *testing.T) {
	for _, test := range []struct {
		in   []string
		want string
	}{
		{
			in:   []string{},
			want: "",
		},
		{
			in:   []string{"TestOne"},
			want: "^TestOne$",
		},
		{
			in:   []string{"TestOne", "TestTwo"},
			want: "^(TestOne|TestTwo)$",
		},
		{
			in:   []string{"TestOne", "TestTwo", "TestThree"},
			want: "^(TestOne|TestThree|TestTwo)$",
		},
		{
			in:   []string{"TestOne/Sub1"},
			want: "^TestOne$/^Sub1$",
		},
		{
			in: []string{
				"TestOne/Sub1",
				"TestTwo",
			},
			want: "^TestOne$/^Sub1$|^TestTwo$",
		},
		{
			in: []string{
				"TestOne/Sub1",
				"TestOne/Sub2",
				"TestTwo",
			},
			want: "^TestOne$/^(Sub1|Sub2)$|^TestTwo$",
		},
		{
			in: []string{
				"TestOne/Sub1",
				"TestOne/Sub2/SubSub1",
				"TestTwo",
			},
			want: "^TestOne$/^Sub1$|^TestOne$/^Sub2$/^SubSub1$|^TestTwo$",
		},
		{
			in: []string{
				"TestTests/A1",
				"TestTests/B/B1",
				"TestTests/C/C3/C31",
			},
			want: "^TestTests$/^A1$|^TestTests$/^B$/^B1$|^TestTests$/^C$/^C3$/^C31$",
		},
	} {
		got := testsToRegexp(test.in)
		assert.Equal(t, test.want, got, fmt.Sprintf("in=%v want=%q got=%q", test.in, test.want, got))
	}
}

var runRe = regexp.MustCompile(`(?m)^\s*=== RUN\s*(Test.*?)\s*$`)

// Test the regexp work with the -run flag in actually selecting the right tests
func TestTestsToRegexpLive(t *testing.T) {
	for _, test := range []struct {
		in   []string
		want []string
	}{
		{
			in: []string{
				"TestTests/A1",
				"TestTests/C/C3",
			},
			want: []string{
				"TestTests",
				"TestTests/A1",
				"TestTests/C",
				"TestTests/C/C3",
				"TestTests/C/C3/C31",
				"TestTests/C/C3/C32",
			},
		},
		{
			in: []string{
				"TestTests",
				"TestTests/A1",
				"TestTests/B",
				"TestTests/B/B1",
				"TestTests/C",
			},
			want: []string{
				"TestTests",
				"TestTests/A1",
				"TestTests/B",
				"TestTests/B/B1",
				"TestTests/C",
				"TestTests/C/C1",
				"TestTests/C/C2",
				"TestTests/C/C3",
				"TestTests/C/C3/C31",
				"TestTests/C/C3/C32",
			},
		},
		{
			in: []string{
				"TestTests/A1",
				"TestTests/B/B1",
				"TestTests/C/C3/C31",
			},
			want: []string{
				"TestTests",
				"TestTests/A1",
				"TestTests/B",
				"TestTests/B/B1",
				"TestTests/C",
				"TestTests/C/C3",
				"TestTests/C/C3/C31",
			},
		},
		{
			in: []string{
				"TestTests/B/B1",
				"TestTests/C/C3/C31",
			},
			want: []string{
				"TestTests",
				"TestTests/B",
				"TestTests/B/B1",
				"TestTests/C",
				"TestTests/C/C3",
				"TestTests/C/C3/C31",
			},
		},
	} {
		runRegexp := testsToRegexp(test.in)
		cmd := exec.Command("go", "test", "-v", "-run", runRegexp)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
		var got []string
		for _, match := range runRe.FindAllSubmatch(out, -1) {
			got = append(got, string(match[1]))
		}
		assert.Equal(t, test.want, got, fmt.Sprintf("in=%v want=%v got=%v, runRegexp=%q", test.in, test.want, got, runRegexp))
	}
}

var nilTest = func(t *testing.T) {}

// Nested tests for TestTestsToRegexpLive to run
func TestTests(t *testing.T) {
	t.Run("A1", nilTest)
	t.Run("A2", nilTest)
	t.Run("B", func(t *testing.T) {
		t.Run("B1", nilTest)
		t.Run("B2", nilTest)
	})
	t.Run("C", func(t *testing.T) {
		t.Run("C1", nilTest)
		t.Run("C2", nilTest)
		t.Run("C3", func(t *testing.T) {
			t.Run("C31", nilTest)
			t.Run("C32", nilTest)
		})
	})
}
