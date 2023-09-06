package log

type Handler interface {
	Handle(r Record)
}

type Record struct {
	Msg
	Level Level
	Names []string
}
