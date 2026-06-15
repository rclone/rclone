package genautocomplete

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	completionDefinition.AddCommand(powershellCommandDefinition)
}

// powerShellInvokeLine is the line in the Cobra generated PowerShell completion
// script that captures rclone's output through a pipeline.
const powerShellInvokeLine = `Invoke-Expression -OutVariable out "$RequestComp" 2>&1 | Out-Null`

// powerShellUTF8Fix forces the captured output to be decoded as UTF-8. When
// PowerShell captures a child process' stdout through a pipeline it decodes the
// bytes using [Console]::OutputEncoding, which on non-UTF-8 systems (for
// example PowerShell 5.1 on a Windows install with an OEM code page such as
// CP852) corrupts the UTF-8 that rclone emits. Setting the encoding to UTF-8 is
// safe on PowerShell 7+, where it is already the default.
const powerShellUTF8Fix = `[Console]::OutputEncoding = [System.Text.Encoding]::UTF8`

// patchPowerShellCompletion injects the UTF-8 output encoding fix immediately
// before the Invoke-Expression call in the Cobra generated PowerShell
// completion script. If the expected line is not found (for example because the
// upstream Cobra template changed), the script is returned unmodified so we
// never emit a corrupted completion script.
func patchPowerShellCompletion(script string) string {
	idx := strings.Index(script, powerShellInvokeLine)
	if idx == -1 {
		return script
	}
	// Reuse the indentation of the Invoke-Expression line for the inserted line.
	lineStart := strings.LastIndex(script[:idx], "\n") + 1
	indent := script[lineStart:idx]
	return script[:lineStart] + indent + powerShellUTF8Fix + "\n" + script[lineStart:]
}

var powershellCommandDefinition = &cobra.Command{
	Use:   "powershell [output_file]",
	Short: `Output powershell completion script for rclone.`,
	Long: `Generate the autocompletion script for powershell.

To load completions in your current shell session:

` + "```console" + `
rclone completion powershell | Out-String | Invoke-Expression
` + "```" + `

To load completions for every new session, add the output of the above command
to your powershell profile.

If output_file is "-" or missing, then the output will be written to stdout.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		var buf bytes.Buffer
		if err := cmd.Root.GenPowerShellCompletion(&buf); err != nil {
			fs.Fatal(nil, fmt.Sprint(err))
		}
		script := patchPowerShellCompletion(buf.String())
		if len(args) == 0 || (len(args) > 0 && args[0] == "-") {
			if _, err := os.Stdout.WriteString(script); err != nil {
				fs.Fatal(nil, fmt.Sprint(err))
			}
			return
		}
		if err := os.WriteFile(args[0], []byte(script), 0644); err != nil {
			fs.Fatal(nil, fmt.Sprint(err))
		}
	},
}
