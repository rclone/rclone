package proton

type Status int

const (
	StatusUp Status = iota
	StatusDown
)

func (s Status) String() string {
	switch s {
	case StatusUp:
		return "up"

	case StatusDown:
		return "down"

	default:
		return "unknown"
	}
}

type StatusObserver func(Status)
