// Package cmd provides torrent backend commands for rclone
package cmd

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/spf13/cobra"
)

// Stats represents torrent statistics
type Stats struct {
	InfoHash      string
	Name          string
	TotalSize     int64
	Progress      float64
	Status        string
	Downloaded    int64
	Uploaded      int64
	DownloadSpeed int64
	UploadSpeed   int64
	Peers         int
	Seeds         int
	AddedAt       time.Time
	CompletedAt   *time.Time
}

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	cmdFlags.StringP("hash", "h", "", "Filter by info hash")
	cmdFlags.StringP("status", "s", "", "Filter by status (waiting|downloading|complete)")
}

var commandDefinition = &cobra.Command{
	Use:   "torrent",
	Short: "Run torrent backend commands",
	Long: `
This command provides read-only access to torrent contents and management
of the torrent backend. It allows viewing statistics, controlling downloads,
and managing the torrent pool.`,
}

// statsCommand
var statsCommand = &cobra.Command{
	Use:   "stats remote:",
	Short: "Show statistics for torrents",
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsDir([]string{args[0]})
		hash, _ := command.Flags().GetString("hash")
		status, _ := command.Flags().GetString("status")
		cmd.Run(false, false, command, func() error {
			return doStats(context.Background(), f, hash, status)
		})
	},
}

// pauseCommand
var pauseCommand = &cobra.Command{
	Use:   "pause remote:",
	Short: "Pause torrent downloads",
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsDir([]string{args[0]})
		hash, _ := command.Flags().GetString("hash")
		cmd.Run(false, false, command, func() error {
			return doPause(context.Background(), f, hash)
		})
	},
}

// resumeCommand
var resumeCommand = &cobra.Command{
	Use:   "resume remote:",
	Short: "Resume torrent downloads",
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsDir([]string{args[0]})
		hash, _ := command.Flags().GetString("hash")
		cmd.Run(false, false, command, func() error {
			return doResume(context.Background(), f, hash)
		})
	},
}

// stopCommand
var stopCommand = &cobra.Command{
	Use:   "stop remote:",
	Short: "Stop and remove torrents",
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsDir([]string{args[0]})
		hash, _ := command.Flags().GetString("hash")
		cmd.Run(false, false, command, func() error {
			return doStop(context.Background(), f, hash)
		})
	},
}

// verifyCommand
var verifyCommand = &cobra.Command{
	Use:   "verify remote:",
	Short: "Verify downloaded data",
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsDir([]string{args[0]})
		hash, _ := command.Flags().GetString("hash")
		cmd.Run(false, false, command, func() error {
			return doVerify(context.Background(), f, hash)
		})
	},
}

// trackersCommand
var trackersCommand = &cobra.Command{
	Use:   "trackers remote:",
	Short: "Show tracker information",
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsDir([]string{args[0]})
		hash, _ := command.Flags().GetString("hash")
		cmd.Run(false, false, command, func() error {
			return doTrackers(context.Background(), f, hash)
		})
	},
}

func init() {
	commandDefinition.AddCommand(statsCommand)
	commandDefinition.AddCommand(pauseCommand)
	commandDefinition.AddCommand(resumeCommand)
	commandDefinition.AddCommand(stopCommand)
	commandDefinition.AddCommand(verifyCommand)
	commandDefinition.AddCommand(trackersCommand)
}

// Command handlers

func doStats(ctx context.Context, f fs.Fs, hash, status string) error {
	if backend, ok := f.(interface {
		Stats(context.Context, string, string) error
	}); ok {
		return backend.Stats(ctx, hash, status)
	}
	return fmt.Errorf("remote does not support stats")
}

func doPause(ctx context.Context, f fs.Fs, hash string) error {
	if backend, ok := f.(interface {
		Pause(context.Context, string) error
	}); ok {
		return backend.Pause(ctx, hash)
	}
	return fmt.Errorf("remote does not support pause")
}

func doResume(ctx context.Context, f fs.Fs, hash string) error {
	if backend, ok := f.(interface {
		Resume(context.Context, string) error
	}); ok {
		return backend.Resume(ctx, hash)
	}
	return fmt.Errorf("remote does not support resume")
}

func doStop(ctx context.Context, f fs.Fs, hash string) error {
	if backend, ok := f.(interface {
		Stop(context.Context, string) error
	}); ok {
		return backend.Stop(ctx, hash)
	}
	return fmt.Errorf("remote does not support stop")
}

func doVerify(ctx context.Context, f fs.Fs, hash string) error {
	if backend, ok := f.(interface {
		Verify(context.Context, string) error
	}); ok {
		return backend.Verify(ctx, hash)
	}
	return fmt.Errorf("remote does not support verify")
}

func doTrackers(ctx context.Context, f fs.Fs, hash string) error {
	if backend, ok := f.(interface {
		Trackers(context.Context, string) error
	}); ok {
		return backend.Trackers(ctx, hash)
	}
	return fmt.Errorf("remote does not support trackers")
}

// formatStats formats statistics for display
func formatStats(w *tabwriter.Writer, stats interface{}) {
	switch s := stats.(type) {
	case *Stats:
		fmt.Fprintf(w, "Hash:\t%s\n", s.InfoHash)
		fmt.Fprintf(w, "Name:\t%s\n", s.Name)
		fmt.Fprintf(w, "Size:\t%s\n", humanize.Bytes(uint64(s.TotalSize)))
		fmt.Fprintf(w, "Progress:\t%.1f%%\n", s.Progress)
		fmt.Fprintf(w, "Status:\t%s\n", s.Status)
		fmt.Fprintf(w, "Downloaded:\t%s\n", humanize.Bytes(uint64(s.Downloaded)))
		fmt.Fprintf(w, "Uploaded:\t%s\n", humanize.Bytes(uint64(s.Uploaded)))
		fmt.Fprintf(w, "Download Speed:\t%s/s\n", humanize.Bytes(uint64(s.DownloadSpeed)))
		fmt.Fprintf(w, "Upload Speed:\t%s/s\n", humanize.Bytes(uint64(s.UploadSpeed)))
		fmt.Fprintf(w, "Peers:\t%d\n", s.Peers)
		fmt.Fprintf(w, "Seeds:\t%d\n", s.Seeds)
		fmt.Fprintf(w, "Added:\t%s\n", s.AddedAt.Format("2006-01-02 15:04:05"))
		if s.CompletedAt != nil {
			fmt.Fprintf(w, "Completed:\t%s\n", s.CompletedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Fprintln(w)
	}
}
