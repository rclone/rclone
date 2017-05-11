package doc

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenYamlDoc(t *testing.T) {
	c := initializeWithRootCmd()
	// Need two commands to run the command alphabetical sort
	cmdEcho.AddCommand(cmdTimes, cmdEchoSub, cmdDeprecated)
	c.AddCommand(cmdPrint, cmdEcho)
	cmdRootWithRun.PersistentFlags().StringVarP(&flags2a, "rootflag", "r", "two", strtwoParentHelp)

	out := new(bytes.Buffer)

	// We generate on s subcommand so we have both subcommands and parents
	if err := GenYaml(cmdEcho, out); err != nil {
		t.Fatal(err)
	}
	found := out.String()

	// Our description
	expected := cmdEcho.Long
	if !strings.Contains(found, expected) {
		t.Errorf("Unexpected response.\nExpecting to contain: \n %q\nGot:\n %q\n", expected, found)
	}

	// Better have our example
	expected = cmdEcho.Example
	if !strings.Contains(found, expected) {
		t.Errorf("Unexpected response.\nExpecting to contain: \n %q\nGot:\n %q\n", expected, found)
	}

	// A local flag
	expected = "boolone"
	if !strings.Contains(found, expected) {
		t.Errorf("Unexpected response.\nExpecting to contain: \n %q\nGot:\n %q\n", expected, found)
	}

	// persistent flag on parent
	expected = "rootflag"
	if !strings.Contains(found, expected) {
		t.Errorf("Unexpected response.\nExpecting to contain: \n %q\nGot:\n %q\n", expected, found)
	}

	// We better output info about our parent
	expected = cmdRootWithRun.Short
	if !strings.Contains(found, expected) {
		t.Errorf("Unexpected response.\nExpecting to contain: \n %q\nGot:\n %q\n", expected, found)
	}

	// And about subcommands
	expected = cmdEchoSub.Short
	if !strings.Contains(found, expected) {
		t.Errorf("Unexpected response.\nExpecting to contain: \n %q\nGot:\n %q\n", expected, found)
	}

	unexpected := cmdDeprecated.Short
	if strings.Contains(found, unexpected) {
		t.Errorf("Unexpected response.\nFound: %v\nBut should not have!!\n", unexpected)
	}
}

func TestGenYamlNoTag(t *testing.T) {
	c := initializeWithRootCmd()
	// Need two commands to run the command alphabetical sort
	cmdEcho.AddCommand(cmdTimes, cmdEchoSub, cmdDeprecated)
	c.AddCommand(cmdPrint, cmdEcho)
	c.DisableAutoGenTag = true
	cmdRootWithRun.PersistentFlags().StringVarP(&flags2a, "rootflag", "r", "two", strtwoParentHelp)
	out := new(bytes.Buffer)

	if err := GenYaml(c, out); err != nil {
		t.Fatal(err)
	}
	found := out.String()

	unexpected := "Auto generated"
	checkStringOmits(t, found, unexpected)

}

func TestGenYamlTree(t *testing.T) {
	cmd := &cobra.Command{
		Use: "do [OPTIONS] arg1 arg2",
	}

	tmpdir, err := ioutil.TempDir("", "test-gen-yaml-tree")
	if err != nil {
		t.Fatalf("Failed to create tmpdir: %s", err.Error())
	}
	defer os.RemoveAll(tmpdir)

	if err := GenYamlTree(cmd, tmpdir); err != nil {
		t.Fatalf("GenYamlTree failed: %s", err.Error())
	}

	if _, err := os.Stat(filepath.Join(tmpdir, "do.yaml")); err != nil {
		t.Fatalf("Expected file 'do.yaml' to exist")
	}
}

func BenchmarkGenYamlToFile(b *testing.B) {
	c := initializeWithRootCmd()
	file, err := ioutil.TempFile("", "")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := GenYaml(c, file); err != nil {
			b.Fatal(err)
		}
	}
}
