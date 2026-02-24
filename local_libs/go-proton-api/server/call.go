package server

import (
	"net/http"
	"net/url"
	"time"
)

type Call struct {
	URL    *url.URL
	Method string
	Status int

	Time     time.Time
	Duration time.Duration

	RequestHeader http.Header
	RequestBody   []byte

	ResponseHeader http.Header
	ResponseBody   []byte
}

type callWatcher struct {
	paths  map[string]struct{}
	callFn func(Call)
}

func newCallWatcher(fn func(Call), paths ...string) callWatcher {
	pathMap := make(map[string]struct{}, len(paths))

	for _, path := range paths {
		pathMap[path] = struct{}{}
	}

	return callWatcher{
		paths:  pathMap,
		callFn: fn,
	}
}

func (watcher *callWatcher) isWatching(path string) bool {
	if len(watcher.paths) == 0 {
		return true
	}

	_, ok := watcher.paths[path]

	return ok
}

func (watcher *callWatcher) publish(call Call) {
	watcher.callFn(call)
}
