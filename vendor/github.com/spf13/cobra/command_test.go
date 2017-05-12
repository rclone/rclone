package cobra

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// test to ensure hidden commands run as intended
func TestHiddenCommandExecutes(t *testing.T) {

	// ensure that outs does not already equal what the command will be setting it
	// to, if it did this test would not actually be testing anything...
	if outs == "hidden" {
		t.Errorf("outs should NOT EQUAL hidden")
	}

	cmdHidden.Execute()

	// upon running the command, the value of outs should now be 'hidden'
	if outs != "hidden" {
		t.Errorf("Hidden command failed to run!")
	}
}

// test to ensure hidden commands do not show up in usage/help text
func TestHiddenCommandIsHidden(t *testing.T) {
	if cmdHidden.IsAvailableCommand() {
		t.Errorf("Hidden command found!")
	}
}

func TestStripFlags(t *testing.T) {
	tests := []struct {
		input  []string
		output []string
	}{
		{
			[]string{"foo", "bar"},
			[]string{"foo", "bar"},
		},
		{
			[]string{"foo", "--bar", "-b"},
			[]string{"foo"},
		},
		{
			[]string{"-b", "foo", "--bar", "bar"},
			[]string{},
		},
		{
			[]string{"-i10", "echo"},
			[]string{"echo"},
		},
		{
			[]string{"-i=10", "echo"},
			[]string{"echo"},
		},
		{
			[]string{"--int=100", "echo"},
			[]string{"echo"},
		},
		{
			[]string{"-ib", "echo", "-bfoo", "baz"},
			[]string{"echo", "baz"},
		},
		{
			[]string{"-i=baz", "bar", "-i", "foo", "blah"},
			[]string{"bar", "blah"},
		},
		{
			[]string{"--int=baz", "-bbar", "-i", "foo", "blah"},
			[]string{"blah"},
		},
		{
			[]string{"--cat", "bar", "-i", "foo", "blah"},
			[]string{"bar", "blah"},
		},
		{
			[]string{"-c", "bar", "-i", "foo", "blah"},
			[]string{"bar", "blah"},
		},
		{
			[]string{"--persist", "bar"},
			[]string{"bar"},
		},
		{
			[]string{"-p", "bar"},
			[]string{"bar"},
		},
	}

	cmdPrint := &Command{
		Use:   "print [string to print]",
		Short: "Print anything to the screen",
		Long:  `an utterly useless command for testing.`,
		Run: func(cmd *Command, args []string) {
			tp = args
		},
	}

	var flagi int
	var flagstr string
	var flagbool bool
	cmdPrint.PersistentFlags().BoolVarP(&flagbool, "persist", "p", false, "help for persistent one")
	cmdPrint.Flags().IntVarP(&flagi, "int", "i", 345, "help message for flag int")
	cmdPrint.Flags().StringVarP(&flagstr, "bar", "b", "bar", "help message for flag string")
	cmdPrint.Flags().BoolVarP(&flagbool, "cat", "c", false, "help message for flag bool")

	for _, test := range tests {
		output := stripFlags(test.input, cmdPrint)
		if !reflect.DeepEqual(test.output, output) {
			t.Errorf("expected: %v, got: %v", test.output, output)
		}
	}
}

func TestDisableFlagParsing(t *testing.T) {
	as := []string{"-v", "-race", "-file", "foo.go"}
	targs := []string{}
	cmdPrint := &Command{
		DisableFlagParsing: true,
		Run: func(cmd *Command, args []string) {
			targs = args
		},
	}
	osargs := []string{"cmd"}
	os.Args = append(osargs, as...)
	err := cmdPrint.Execute()
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(as, targs) {
		t.Errorf("expected: %v, got: %v", as, targs)
	}
}

func TestInitHelpFlagMergesFlags(t *testing.T) {
	usage := "custom flag"
	baseCmd := Command{Use: "testcmd"}
	baseCmd.PersistentFlags().Bool("help", false, usage)
	cmd := Command{Use: "do"}
	baseCmd.AddCommand(&cmd)

	cmd.InitDefaultHelpFlag()
	actual := cmd.Flags().Lookup("help").Usage
	if actual != usage {
		t.Fatalf("Expected the help flag from the base command with usage '%s', but got the default with usage '%s'", usage, actual)
	}
}

func TestCommandsAreSorted(t *testing.T) {
	EnableCommandSorting = true

	originalNames := []string{"middle", "zlast", "afirst"}
	expectedNames := []string{"afirst", "middle", "zlast"}

	var tmpCommand = &Command{Use: "tmp"}

	for _, name := range originalNames {
		tmpCommand.AddCommand(&Command{Use: name})
	}

	for i, c := range tmpCommand.Commands() {
		if expectedNames[i] != c.Name() {
			t.Errorf("expected: %s, got: %s", expectedNames[i], c.Name())
		}
	}

	EnableCommandSorting = true
}

func TestEnableCommandSortingIsDisabled(t *testing.T) {
	EnableCommandSorting = false

	originalNames := []string{"middle", "zlast", "afirst"}

	var tmpCommand = &Command{Use: "tmp"}

	for _, name := range originalNames {
		tmpCommand.AddCommand(&Command{Use: name})
	}

	for i, c := range tmpCommand.Commands() {
		if originalNames[i] != c.Name() {
			t.Errorf("expected: %s, got: %s", originalNames[i], c.Name())
		}
	}

	EnableCommandSorting = true
}

func TestSetOutput(t *testing.T) {
	cmd := &Command{}
	cmd.SetOutput(nil)
	if out := cmd.OutOrStdout(); out != os.Stdout {
		t.Fatalf("expected setting output to nil to revert back to stdout, got %v", out)
	}
}

func TestFlagErrorFunc(t *testing.T) {
	cmd := &Command{
		Use: "print",
		RunE: func(cmd *Command, args []string) error {
			return nil
		},
	}
	expectedFmt := "This is expected: %s"

	cmd.SetFlagErrorFunc(func(c *Command, err error) error {
		return fmt.Errorf(expectedFmt, err)
	})
	cmd.SetArgs([]string{"--bogus-flag"})
	cmd.SetOutput(new(bytes.Buffer))

	err := cmd.Execute()

	expected := fmt.Sprintf(expectedFmt, "unknown flag: --bogus-flag")
	if err.Error() != expected {
		t.Errorf("expected %v, got %v", expected, err.Error())
	}
}

// TestSortedFlags checks,
// if cmd.LocalFlags() is unsorted when cmd.Flags().SortFlags set to false.
// Related to https://github.com/spf13/cobra/issues/404.
func TestSortedFlags(t *testing.T) {
	cmd := &Command{}
	cmd.Flags().SortFlags = false
	names := []string{"C", "B", "A", "D"}
	for _, name := range names {
		cmd.Flags().Bool(name, false, "")
	}

	i := 0
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if i == len(names) {
			return
		}
		if isStringInStringSlice(f.Name, names) {
			if names[i] != f.Name {
				t.Errorf("Incorrect order. Expected %v, got %v", names[i], f.Name)
			}
			i++
		}
	})
}

// contains checks, if s is in ss.
func isStringInStringSlice(s string, ss []string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// TestHelpFlagInHelp checks,
// if '--help' flag is shown in help for child (executing `parent help child`),
// that has no other flags.
// Related to https://github.com/spf13/cobra/issues/302.
func TestHelpFlagInHelp(t *testing.T) {
	output := new(bytes.Buffer)
	parent := &Command{Use: "parent", Run: func(*Command, []string) {}}
	parent.SetOutput(output)

	child := &Command{Use: "child", Run: func(*Command, []string) {}}
	parent.AddCommand(child)

	parent.SetArgs([]string{"help", "child"})
	err := parent.Execute()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "[flags]") {
		t.Errorf("\nExpecting to contain: %v\nGot: %v", "[flags]", output.String())
	}
}

// TestMergeCommandLineToFlags checks,
// if pflag.CommandLine is correctly merged to c.Flags() after first call
// of c.mergePersistentFlags.
// Related to https://github.com/spf13/cobra/issues/443.
func TestMergeCommandLineToFlags(t *testing.T) {
	pflag.Bool("boolflag", false, "")
	c := &Command{Use: "c", Run: func(*Command, []string) {}}
	c.mergePersistentFlags()
	if c.Flags().Lookup("boolflag") == nil {
		t.Fatal("Expecting to have flag from CommandLine in c.Flags()")
	}

	// Reset pflag.CommandLine flagset.
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
}
