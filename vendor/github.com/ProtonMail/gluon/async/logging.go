package async

import (
	"context"

	"github.com/ProtonMail/gluon/logging"
)

func GoAnnotated(ctx context.Context, panicHandler PanicHandler, fn func(context.Context), labelMap ...logging.Labels) {
	go func() {
		defer HandlePanic(panicHandler)
		logging.DoAnnotated(ctx, fn, labelMap...)
	}()
}
