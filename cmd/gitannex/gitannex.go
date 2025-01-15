// Package gitannex provides the "gitannex" command, which enables [git-annex]
// to communicate with rclone by implementing the [external special remote
// protocol]. The protocol is line delimited and spoken over stdin and stdout.
//
// # Milestones
//
// (Tracked in [issue #7625].)
//
//  1. ✅ Minimal support for the [external special remote protocol]. Tested on
//     "local" and "drive" backends.
//  2. Add support for the ASYNC protocol extension. This may improve performance.
//  3. Support the [simple export interface]. This will enable `git-annex
//     export` functionality.
//  4. Once the draft is finalized, support import/export interface.
//
// [git-annex]: https://git-annex.branchable.com/
// [external special remote protocol]: https://git-annex.branchable.com/design/external_special_remote_protocol/
// [simple export interface]: https://git-annex.branchable.com/design/external_special_remote_protocol/export_and_import_appendix/
// [issue #7625]: https://github.com/rclone/rclone/issues/7625
package gitannex

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

const subcommandName string = "gitannex"
const uniqueCommandName string = "git-annex-remote-rclone-builtin"

//go:embed gitannex.md
var gitannexHelp string

func init() {
	os.Args = maybeTransformArgs(os.Args)
	cmd.Root.AddCommand(command)
}

// maybeTransformArgs returns a modified version of `args` with the "gitannex"
// subcommand inserted when `args` indicates that the program was executed as
// "git-annex-remote-rclone-builtin". One way this can happen is when rclone is
// invoked via symlink. Otherwise, returns `args`.
func maybeTransformArgs(args []string) []string {
	if len(args) == 0 || filepath.Base(args[0]) != uniqueCommandName {
		return args
	}
	newArgs := make([]string, 0, len(args)+1)
	newArgs = append(newArgs, args[0])
	newArgs = append(newArgs, subcommandName)
	newArgs = append(newArgs, args[1:]...)
	return newArgs
}

// messageParser helps parse messages we receive from git-annex into a sequence
// of parameters. Messages are not quite trivial to parse because they are
// separated by spaces, but the final parameter may itself contain spaces.
//
// This abstraction is necessary because simply splitting on space doesn't cut
// it. Also, we cannot know how many parameters to parse until we've parsed the
// first parameter.
type messageParser struct {
	line string
}

// nextSpaceDelimitedParameter consumes the next space-delimited parameter.
func (m *messageParser) nextSpaceDelimitedParameter() (string, error) {
	m.line = strings.TrimRight(m.line, "\r\n")
	if len(m.line) == 0 {
		return "", errors.New("nothing remains to parse")
	}

	before, after, found := strings.Cut(m.line, " ")
	if found {
		if len(before) == 0 {
			return "", fmt.Errorf("found an empty space-delimited parameter in line: %q", m.line)
		}
		m.line = after
		return before, nil
	}

	remaining := m.line
	m.line = ""
	return remaining, nil
}

// finalParameter consumes the final parameter, which may contain spaces.
func (m *messageParser) finalParameter() string {
	m.line = strings.TrimRight(m.line, "\r\n")
	if len(m.line) == 0 {
		return ""
	}

	param := m.line
	m.line = ""
	return param
}

// configDefinition describes a configuration value required by this command. We
// use "GETCONFIG" messages to query git-annex for these values at runtime.
type configDefinition struct {
	names        []string
	description  string
	destination  *string
	defaultValue *string
}

func (c *configDefinition) getCanonicalName() string {
	if len(c.names) < 1 {
		panic(fmt.Errorf("configDefinition must have at least one name: %v", c))
	}
	return c.names[0]
}

// fullDescription returns a single-line, human-readable description for this
// config. The returned string begins with a list of synonyms and ends with
// `c.description`.
func (c *configDefinition) fullDescription() string {
	if len(c.names) <= 1 {
		return c.description
	}
	// Exclude the canonical name from the list of synonyms.
	synonyms := c.names[1:len(c.names)]
	commaSeparatedSynonyms := strings.Join(synonyms, ", ")
	return fmt.Sprintf("(synonyms: %s) %s", commaSeparatedSynonyms, c.description)
}

// server contains this command's current state.
type server struct {
	reader *bufio.Reader
	writer io.Writer

	// When true, the server prints a transcript of messages sent and received
	// to stderr.
	verbose bool

	extensionInfo                bool
	extensionAsync               bool
	extensionGetGitRemoteName    bool
	extensionUnavailableResponse bool

	configsDone            bool
	configPrefix           string
	configRcloneRemoteName string
	configRcloneLayout     string
}

func (s *server) sendMsg(msg string) {
	msg += "\n"
	if _, err := io.WriteString(s.writer, msg); err != nil {
		panic(err)
	}
	if s.verbose {
		_, err := os.Stderr.WriteString(fmt.Sprintf("server sent %q\n", msg))
		if err != nil {
			panic(fmt.Errorf("failed to write verbose message to stderr: %w", err))
		}
	}
}

func (s *server) getMsg() (*messageParser, error) {
	msg, err := s.reader.ReadString('\n')
	if err != nil {
		if len(msg) == 0 {
			// Git-annex closes stdin when it is done with us, so failing to
			// read a new line is not an error.
			return nil, nil
		}
		return nil, fmt.Errorf("expected message to end with newline: %q", msg)
	}
	if s.verbose {
		_, err := os.Stderr.WriteString(fmt.Sprintf("server received %q\n", msg))
		if err != nil {
			return nil, fmt.Errorf("failed to write verbose message to stderr: %w", err)
		}
	}
	return &messageParser{msg}, nil
}

func (s *server) run() error {
	// The remote sends the first message.
	s.sendMsg("VERSION 1")

	for {
		message, err := s.getMsg()
		if err != nil {
			return fmt.Errorf("error receiving message: %w", err)
		}

		if message == nil {
			break
		}

		command, err := message.nextSpaceDelimitedParameter()
		if err != nil {
			return fmt.Errorf("failed to parse command")
		}

		switch command {
		//
		// Git-annex requires that these requests are supported.
		//
		case "INITREMOTE":
			err = s.handleInitRemote()
		case "PREPARE":
			err = s.handlePrepare()
		case "EXPORTSUPPORTED":
			// Indicate that we do not support exports.
			s.sendMsg("EXPORTSUPPORTED-FAILURE")
		case "TRANSFER":
			err = s.handleTransfer(message)
		case "CHECKPRESENT":
			err = s.handleCheckPresent(message)
		case "REMOVE":
			err = s.handleRemove(message)
		case "ERROR":
			errorMessage := message.finalParameter()
			err = fmt.Errorf("received error message from git-annex: %s", errorMessage)

		//
		// These requests are optional.
		//
		case "EXTENSIONS":
			// Git-annex just told us which protocol extensions it supports.
			// Respond with the list of extensions that we want to use (none).
			err = s.handleExtensions(message)
		case "LISTCONFIGS":
			s.handleListConfigs()
		case "GETCOST":
			// Git-annex wants to know the "cost" of using this remote. It
			// probably depends on the backend we will be using, but let's just
			// consider this an "expensive remote" per git-annex's
			// Config/Cost.hs.
			s.sendMsg("COST 200")
		case "GETAVAILABILITY":
			// Indicate that this is a cloud service.
			s.sendMsg("AVAILABILITY GLOBAL")
		case "CLAIMURL", "CHECKURL", "WHEREIS", "GETINFO":
			s.sendMsg("UNSUPPORTED-REQUEST")
		default:
			err = fmt.Errorf("received unexpected message from git-annex: %s", message.line)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// Idempotently handle an incoming INITREMOTE message. This should perform
// one-time setup operations, but we may receive the command again, e.g. when
// this git-annex remote is initialized in a different repository.
func (s *server) handleInitRemote() error {
	if err := s.queryConfigs(); err != nil {
		return fmt.Errorf("failed to get configs: %w", err)
	}

	remoteRootFs, err := cache.Get(context.TODO(), fmt.Sprintf("%s:", s.configRcloneRemoteName))
	if err != nil {
		s.sendMsg("INITREMOTE-FAILURE failed to open root directory of rclone remote")
		return fmt.Errorf("failed to open root directory of rclone remote: %w", err)
	}

	if !remoteRootFs.Features().CanHaveEmptyDirectories {
		s.sendMsg("INITREMOTE-FAILURE this rclone remote does not support empty directories")
		return fmt.Errorf("rclone remote does not support empty directories")
	}

	if err := operations.Mkdir(context.TODO(), remoteRootFs, s.configPrefix); err != nil {
		s.sendMsg("INITREMOTE-FAILURE failed to mkdir")
		return fmt.Errorf("failed to mkdir: %w", err)
	}

	s.sendMsg("INITREMOTE-SUCCESS")
	return nil
}

// Get a list of configs with pointers to fields of `s`.
func (s *server) getRequiredConfigs() []configDefinition {
	defaultRclonePrefix := "git-annex-rclone"
	defaultRcloneLayout := "nodir"

	return []configDefinition{
		{
			[]string{"rcloneremotename", "target"},
			"Name of the rclone remote to use. " +
				"Must match a remote known to rclone. " +
				"(Note that rclone remotes are a distinct concept from git-annex remotes.)",
			&s.configRcloneRemoteName,
			nil,
		},
		{
			[]string{"rcloneprefix", "prefix"},
			"Directory where rclone will write git-annex content. " +
				fmt.Sprintf("If not specified, defaults to %q. ", defaultRclonePrefix) +
				"This directory will be created on init if it does not exist.",
			&s.configPrefix,
			&defaultRclonePrefix,
		},
		{
			[]string{"rclonelayout", "rclone_layout"},
			"Defines where, within the rcloneprefix directory, rclone will write git-annex content. " +
				fmt.Sprintf("Must be one of %v. ", allLayoutModes()) +
				fmt.Sprintf("If empty, defaults to %q.", defaultRcloneLayout),
			&s.configRcloneLayout,
			&defaultRcloneLayout,
		},
	}
}

// Query git-annex for config values.
func (s *server) queryConfigs() error {
	if s.configsDone {
		return nil
	}

	// Send a "GETCONFIG" message for each required config and parse git-annex's
	// "VALUE" response.
	for _, config := range s.getRequiredConfigs() {
		var valueReceived bool
		// Try each of the config's names in sequence, starting with the
		// canonical name.
		for _, configName := range config.names {
			s.sendMsg(fmt.Sprintf("GETCONFIG %s", configName))

			message, err := s.getMsg()
			if err != nil {
				return err
			}

			valueKeyword, err := message.nextSpaceDelimitedParameter()
			if err != nil || valueKeyword != "VALUE" {
				return fmt.Errorf("failed to parse config value: %s %s", valueKeyword, message.line)
			}

			value := message.finalParameter()
			if value != "" {
				*config.destination = value
				valueReceived = true
				break
			}
		}
		if !valueReceived {
			if config.defaultValue == nil {
				return fmt.Errorf("did not receive a non-empty config value for %q", config.getCanonicalName())
			}
			*config.destination = *config.defaultValue
		}
	}

	s.configsDone = true
	return nil
}

func (s *server) handlePrepare() error {
	if err := s.queryConfigs(); err != nil {
		s.sendMsg("PREPARE-FAILURE Error getting configs")
		return fmt.Errorf("error getting configs: %w", err)
	}
	s.sendMsg("PREPARE-SUCCESS")
	return nil
}

// Git-annex is asking us to return the list of settings that we use. Keep this
// in sync with `handlePrepare()`.
func (s *server) handleListConfigs() {
	for _, config := range s.getRequiredConfigs() {
		s.sendMsg(fmt.Sprintf("CONFIG %s %s", config.getCanonicalName(), config.fullDescription()))
	}
	s.sendMsg("CONFIGEND")
}

func (s *server) handleTransfer(message *messageParser) error {
	argMode, err := message.nextSpaceDelimitedParameter()
	if err != nil {
		s.sendMsg("TRANSFER-FAILURE failed to parse direction")
		return fmt.Errorf("malformed arguments for TRANSFER: %w", err)
	}
	argKey, err := message.nextSpaceDelimitedParameter()
	if err != nil {
		s.sendMsg("TRANSFER-FAILURE failed to parse key")
		return fmt.Errorf("malformed arguments for TRANSFER: %w", err)
	}
	argFile := message.finalParameter()
	if argFile == "" {
		s.sendMsg("TRANSFER-FAILURE failed to parse file path")
		return errors.New("failed to parse file path")
	}

	if err := s.queryConfigs(); err != nil {
		s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s %s failed to get configs", argMode, argKey))
		return fmt.Errorf("error getting configs: %w", err)
	}

	layout := parseLayoutMode(s.configRcloneLayout)
	if layout == layoutModeUnknown {
		s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s", argKey))
		return fmt.Errorf("error parsing layout mode: %q", s.configRcloneLayout)
	}

	remoteFsString, err := buildFsString(s.queryDirhash, layout, argKey, s.configRcloneRemoteName, s.configPrefix)
	if err != nil {
		s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s", argKey))
		return fmt.Errorf("error building fs string: %w", err)
	}

	remoteFs, err := cache.Get(context.TODO(), remoteFsString)
	if err != nil {
		s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s %s failed to get remote fs", argMode, argKey))
		return err
	}

	localDir := filepath.Dir(argFile)
	localFs, err := cache.Get(context.TODO(), localDir)
	if err != nil {
		s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s %s failed to get local fs", argMode, argKey))
		return fmt.Errorf("failed to get local fs: %w", err)
	}

	remoteFileName := argKey
	localFileName := filepath.Base(argFile)

	switch argMode {
	case "STORE":
		err = operations.CopyFile(context.TODO(), remoteFs, localFs, remoteFileName, localFileName)
		if err != nil {
			s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s %s failed to copy file: %s", argMode, argKey, err))
			return err
		}

	case "RETRIEVE":
		err = operations.CopyFile(context.TODO(), localFs, remoteFs, localFileName, remoteFileName)
		// It is non-fatal when retrieval fails because the file is missing on
		// the remote.
		if err == fs.ErrorObjectNotFound {
			s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s %s not found", argMode, argKey))
			return nil
		}
		if err != nil {
			s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s %s failed to copy file: %s", argMode, argKey, err))
			return err
		}

	default:
		s.sendMsg(fmt.Sprintf("TRANSFER-FAILURE %s %s unrecognized mode", argMode, argKey))
		return fmt.Errorf("received malformed TRANSFER mode: %v", argMode)
	}

	s.sendMsg(fmt.Sprintf("TRANSFER-SUCCESS %s %s", argMode, argKey))
	return nil
}

func (s *server) handleCheckPresent(message *messageParser) error {
	argKey := message.finalParameter()
	if argKey == "" {
		return errors.New("failed to parse response for CHECKPRESENT")
	}

	if err := s.queryConfigs(); err != nil {
		s.sendMsg(fmt.Sprintf("CHECKPRESENT-FAILURE %s failed to get configs", argKey))
		return fmt.Errorf("error getting configs: %s", err)
	}

	layout := parseLayoutMode(s.configRcloneLayout)
	if layout == layoutModeUnknown {
		s.sendMsg(fmt.Sprintf("CHECKPRESENT-FAILURE %s", argKey))
		return fmt.Errorf("error parsing layout mode: %q", s.configRcloneLayout)
	}

	remoteFsString, err := buildFsString(s.queryDirhash, layout, argKey, s.configRcloneRemoteName, s.configPrefix)
	if err != nil {
		s.sendMsg(fmt.Sprintf("CHECKPRESENT-FAILURE %s", argKey))
		return fmt.Errorf("error building fs string: %w", err)
	}

	remoteFs, err := cache.Get(context.TODO(), remoteFsString)
	if err != nil {
		s.sendMsg(fmt.Sprintf("CHECKPRESENT-UNKNOWN %s failed to get remote fs", argKey))
		return err
	}

	_, err = remoteFs.NewObject(context.TODO(), argKey)
	if err == fs.ErrorObjectNotFound {
		s.sendMsg(fmt.Sprintf("CHECKPRESENT-FAILURE %s", argKey))
		return nil
	}
	if err != nil {
		s.sendMsg(fmt.Sprintf("CHECKPRESENT-UNKNOWN %s error finding file", argKey))
		return err
	}

	s.sendMsg(fmt.Sprintf("CHECKPRESENT-SUCCESS %s", argKey))
	return nil
}

func (s *server) queryDirhash(msg string) (string, error) {
	s.sendMsg(msg)
	parser, err := s.getMsg()
	if err != nil {
		return "", err
	}
	keyword, err := parser.nextSpaceDelimitedParameter()
	if err != nil {
		return "", err
	}
	if keyword != "VALUE" {
		return "", fmt.Errorf("expected VALUE keyword, but got %q", keyword)
	}
	dirhash, err := parser.nextSpaceDelimitedParameter()
	if err != nil {
		return "", fmt.Errorf("failed to parse dirhash: %w", err)
	}
	return dirhash, nil
}

func (s *server) handleRemove(message *messageParser) error {
	argKey := message.finalParameter()
	if argKey == "" {
		return errors.New("failed to parse key for REMOVE")
	}

	layout := parseLayoutMode(s.configRcloneLayout)
	if layout == layoutModeUnknown {
		s.sendMsg(fmt.Sprintf("REMOVE-FAILURE %s", argKey))
		return fmt.Errorf("error parsing layout mode: %q", s.configRcloneLayout)
	}

	remoteFsString, err := buildFsString(s.queryDirhash, layout, argKey, s.configRcloneRemoteName, s.configPrefix)
	if err != nil {
		s.sendMsg(fmt.Sprintf("REMOVE-FAILURE %s", argKey))
		return fmt.Errorf("error building fs string: %w", err)
	}

	remoteFs, err := cache.Get(context.TODO(), remoteFsString)
	if err != nil {
		s.sendMsg(fmt.Sprintf("REMOVE-FAILURE %s", argKey))
		return fmt.Errorf("error getting remote fs: %w", err)
	}

	fileObj, err := remoteFs.NewObject(context.TODO(), argKey)
	// It is non-fatal when removal fails because the file is missing on the
	// remote.
	if errors.Is(err, fs.ErrorObjectNotFound) {
		s.sendMsg(fmt.Sprintf("REMOVE-SUCCESS %s", argKey))
		return nil
	}
	if err != nil {
		s.sendMsg(fmt.Sprintf("REMOVE-FAILURE %s error getting new fs object: %s", argKey, err))
		return fmt.Errorf("error getting new fs object: %w", err)
	}
	if err := operations.DeleteFile(context.TODO(), fileObj); err != nil {
		s.sendMsg(fmt.Sprintf("REMOVE-FAILURE %s error deleting file", argKey))
		return fmt.Errorf("error deleting file: %q", argKey)
	}
	s.sendMsg(fmt.Sprintf("REMOVE-SUCCESS %s", argKey))
	return nil
}

func (s *server) handleExtensions(message *messageParser) error {
	for {
		extension, err := message.nextSpaceDelimitedParameter()
		if err != nil {
			break
		}
		switch extension {
		case "INFO":
			s.extensionInfo = true
		case "ASYNC":
			s.extensionAsync = true
		case "GETGITREMOTENAME":
			s.extensionGetGitRemoteName = true
		case "UNAVAILABLERESPONSE":
			s.extensionUnavailableResponse = true
		}
	}
	s.sendMsg("EXTENSIONS")
	return nil
}

var command = &cobra.Command{
	Aliases: []string{uniqueCommandName},
	Use:     subcommandName,
	Short:   "Speaks with git-annex over stdin/stdout.",
	Long:    gitannexHelp,
	Annotations: map[string]string{
		"versionIntroduced": "v1.67.0",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)

		s := server{
			reader: bufio.NewReader(os.Stdin),
			writer: os.Stdout,
		}
		err := s.run()
		if err != nil {
			s.sendMsg(fmt.Sprintf("ERROR %s", err.Error()))
			panic(err)
		}
	},
}
