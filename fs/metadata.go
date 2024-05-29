package fs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Metadata represents Object metadata in a standardised form
//
// See docs/content/metadata.md for the interpretation of the keys
type Metadata map[string]string

// MetadataHelp represents help for a bit of system metadata
type MetadataHelp struct {
	Help     string
	Type     string
	Example  string
	ReadOnly bool
}

// MetadataInfo is help for the whole metadata for this backend.
type MetadataInfo struct {
	System map[string]MetadataHelp
	Help   string
}

// Set k to v on m
//
// If m is nil, then it will get made
func (m *Metadata) Set(k, v string) {
	if *m == nil {
		*m = make(Metadata, 1)
	}
	(*m)[k] = v
}

// Merge other into m
//
// If m is nil, then it will get made
func (m *Metadata) Merge(other Metadata) {
	for k, v := range other {
		if *m == nil {
			*m = make(Metadata, len(other))
		}
		(*m)[k] = v
	}
}

// MergeOptions gets any Metadata from the options passed in and
// stores it in m (which may be nil).
//
// If there is no m then metadata will be nil
func (m *Metadata) MergeOptions(options []OpenOption) {
	for _, opt := range options {
		if metadataOption, ok := opt.(MetadataOption); ok {
			m.Merge(Metadata(metadataOption))
		}
	}
}

// GetMetadata from an DirEntry
//
// If the object has no metadata then metadata will be nil
func GetMetadata(ctx context.Context, o DirEntry) (metadata Metadata, err error) {
	do, ok := o.(Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}

// mapItem descripts the item to be mapped
type mapItem struct {
	SrcFs     string
	SrcFsType string
	DstFs     string
	DstFsType string
	Remote    string
	Size      int64
	MimeType  string `json:",omitempty"`
	ModTime   time.Time
	IsDir     bool
	ID        string   `json:",omitempty"`
	Metadata  Metadata `json:",omitempty"`
}

// This runs an external program on the metadata which can be used to
// map it from one form to another.
func metadataMapper(ctx context.Context, cmdLine SpaceSepList, dstFs Fs, o DirEntry, metadata Metadata) (newMetadata Metadata, err error) {
	ci := GetConfig(ctx)
	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)
	in := mapItem{
		DstFs:     ConfigString(dstFs),
		DstFsType: Type(dstFs),
		Remote:    o.Remote(),
		Size:      o.Size(),
		MimeType:  MimeType(ctx, o),
		ModTime:   o.ModTime(ctx),
		IsDir:     false,
		Metadata:  metadata,
	}
	fInfo := o.Fs()
	if f, ok := fInfo.(Fs); ok {
		in.SrcFs = ConfigString(f)
		in.SrcFsType = Type(f)
	} else {
		in.SrcFs = fInfo.Name() + ":" + fInfo.Root()
		in.SrcFsType = "unknown"
	}
	if do, ok := o.(IDer); ok {
		in.ID = do.ID()
	}
	inBytes, err := json.MarshalIndent(in, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("metadata mapper: failed to marshal input: %w", err)
	}
	if ci.Dump.IsSet(DumpMapper) {
		Debugf(nil, "Metadata mapper sent: \n%s\n", string(inBytes))
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewBuffer(inBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err = cmd.Run()
	Debugf(o, "Calling metadata mapper %v", cmdLine)
	duration := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("metadata mapper: failed on %v: %q: %w", cmdLine, strings.TrimSpace(stderr.String()), err)
	}
	if ci.Dump.IsSet(DumpMapper) {
		Debugf(nil, "Metadata mapper received: \n%s\n", stdout.String())
	}
	var out mapItem
	err = json.Unmarshal(stdout.Bytes(), &out)
	if err != nil {
		return nil, fmt.Errorf("metadata mapper: failed to read output: %q: %w", stdout.String(), err)
	}
	Debugf(o, "Metadata mapper returned in %v", duration)
	return out.Metadata, nil
}

// GetMetadataOptions from an DirEntry and merge it with any in options
//
// If --metadata isn't in use it will return nil.
//
// If the object has no metadata then metadata will be nil.
//
// This should be passed the destination Fs for the metadata mapper
func GetMetadataOptions(ctx context.Context, dstFs Fs, o DirEntry, options []OpenOption) (metadata Metadata, err error) {
	ci := GetConfig(ctx)
	if !ci.Metadata {
		return nil, nil
	}
	metadata, err = GetMetadata(ctx, o)
	if err != nil {
		return nil, err
	}
	metadata.MergeOptions(options)
	if len(ci.MetadataMapper) != 0 {
		metadata, err = metadataMapper(ctx, ci.MetadataMapper, dstFs, o, metadata)
		if err != nil {
			return nil, err
		}
	}
	return metadata, nil
}
