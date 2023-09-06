package async

import "sync"

type WaitGroup struct {
	wg           sync.WaitGroup
	panicHandler PanicHandler
}

func MakeWaitGroup(panicHandler PanicHandler) WaitGroup {
	return WaitGroup{panicHandler: panicHandler}
}

func (wg *WaitGroup) Go(f func()) {
	wg.wg.Add(1)

	go func() {
		defer HandlePanic(wg.panicHandler)

		defer wg.wg.Done()
		f()
	}()
}

func (wg *WaitGroup) Wait() {
	wg.wg.Wait()
}
