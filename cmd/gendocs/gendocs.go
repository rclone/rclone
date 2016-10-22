package gendocs

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
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
		out := args[0]
		err := os.MkdirAll(out, 0777)
		if err != nil {
			return err
		}
		now := time.Now().Format(time.RFC3339)
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
		return doc.GenMarkdownTreeCustom(cmd.Root, out, prepender, linkHandler)
	},
}
