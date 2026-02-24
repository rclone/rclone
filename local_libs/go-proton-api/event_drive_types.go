package proton

type DriveEvent struct {
	EventID string

	Events []LinkEvent

	Refresh Bool
}

type LinkEvent struct {
	EventID string

	EventType LinkEventType

	CreateTime int

	Link Link

	Data any
}

type LinkEventType int

const (
	LinkEventDelete LinkEventType = iota
	LinkEventCreate
	LinkEventUpdate
	LinkEventUpdateMetadata
)
