package proton

import (
	"encoding/json"
	"strings"
)

const (
	InboxLabel        = "0"
	AllDraftsLabel    = "1"
	AllSentLabel      = "2"
	TrashLabel        = "3"
	SpamLabel         = "4"
	AllMailLabel      = "5"
	ArchiveLabel      = "6"
	SentLabel         = "7"
	DraftsLabel       = "8"
	OutboxLabel       = "9"
	StarredLabel      = "10"
	AllScheduledLabel = "12"
)

type Label struct {
	ID       string
	ParentID string

	Name  string
	Path  []string
	Color string
	Type  LabelType
}

func (label *Label) UnmarshalJSON(data []byte) error {
	type Alias Label

	aux := &struct {
		Path string

		*Alias
	}{
		Alias: (*Alias)(label),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	label.Path = strings.Split(aux.Path, "/")

	return nil
}

func (label Label) MarshalJSON() ([]byte, error) {
	type Alias Label

	aux := &struct {
		Path string

		*Alias
	}{
		Path:  strings.Join(label.Path, "/"),
		Alias: (*Alias)(&label),
	}

	return json.Marshal(aux)
}

type CreateLabelReq struct {
	Name  string
	Color string
	Type  LabelType

	ParentID string `json:",omitempty"`
}

type UpdateLabelReq struct {
	Name  string
	Color string

	ParentID string `json:",omitempty"`
}

type LabelType int

const (
	LabelTypeLabel LabelType = iota + 1
	LabelTypeContactGroup
	LabelTypeFolder
	LabelTypeSystem
)
