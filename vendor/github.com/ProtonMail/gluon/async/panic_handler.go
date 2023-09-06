package async

type PanicHandler interface {
	HandlePanic(interface{})
}

type NoopPanicHandler struct{}

func (n NoopPanicHandler) HandlePanic(r interface{}) {}

func HandlePanic(panicHandler PanicHandler) {
	if panicHandler == nil {
		return
	}

	if _, ok := panicHandler.(NoopPanicHandler); ok {
		return
	}

	if _, ok := panicHandler.(*NoopPanicHandler); ok {
		return
	}

	panicHandler.HandlePanic(recover())
}
