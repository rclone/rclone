package gendocs

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

const gendocFrontmatterTemplate = `---
date: %s
title: "%s"
slug: %s
url: %s
---
`

var commandDefintion = &cobra.Command{
	Use:   "gendocs output_directory",
	Short: `Output markdown docs for rclone to the directory supplied.`,
	Long: `
This produces markdown docs for the rclone commands to the directory
supplied.  These are in a format suitable for hugo to render into the
rclone.org website.`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		now := time.Now().Format(time.RFC3339)

		// Create the directory structure
		root := args[0]
		out := filepath.Join(root, "commands")
		err := os.MkdirAll(out, 0777)
		if err != nil {
			return err
		}

		// Write the flags page
		var buf bytes.Buffer
		cmd.Root.SetOutput(&buf)
		cmd.Root.SetArgs([]string{"help", "flags"})
		cmd.GeneratingDocs = true
		err = cmd.Root.Execute()
		if err != nil {
			return err
		}
		flagsHelp := strings.Replace(buf.String(), "YYYY-MM-DD", now, -1)
		err = ioutil.WriteFile(filepath.Join(root, "flags.md"), []byte(flagsHelp), 0777)
		if err != nil {
			return err
		}

		// markup for the docs files
		prepender := func(filename string) string {
			name := filepath.Base(filename)
			base := strings.TrimSuffix(name, path.Ext(name))
			url := "/commands/" + strings.ToLower(base) + "/"
			return fmt.Sprintf(gendocFrontmatterTemplate, now, strings.Replace(base, "_", " ", -1), base, url)
		}
		linkHandler := func(name string) string {
			base := strings.TrimSuffix(name, path.Ext(name))
			return "/commands/" + strings.ToLower(base) + "/"
		}

		// Hide all of the root entries flags
		cmd.Root.Flags().VisitAll(func(flag *pflag.Flag) {
			flag.Hidden = true
		})
		err = doc.GenMarkdownTreeCustom(cmd.Root, out, prepender, linkHandler)
		if err != nil {
			return err
		}

		// Munge the files to add a link to the global flags page
		err = filepath.Walk(out, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				b, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				doc := string(b)
				doc = strings.Replace(doc, "\n### SEE ALSO", `
See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO`, 1)
				err = ioutil.WriteFile(path, []byte(doc), 0777)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	},
}
