package logging

import (
	"context"
	"fmt"
	"runtime"
	"runtime/pprof"
)

type Labels map[string]any

const (
	// FnKey refers to the function name.
	FnKey = "fn"

	// FileKey refers to the file name.
	FileKey = "file"

	// LineKey refers to the line number.
	LineKey = "line"
)

func DoAnnotated(ctx context.Context, fn func(context.Context), labelMap ...Labels) {
	pprofDo(ctx, toLabelSet(labelMap...), fn)
}

func toLabelSet(labelMap ...Labels) pprof.LabelSet {
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		panic("failed to get caller's stack frame")
	}

	var labels []string

	for _, labelMap := range append(labelMap, getDefaultLabels(pc, file, line)) {
		for key, val := range labelMap {
			labels = append(labels, key, fmt.Sprintf("%v", val))
		}
	}

	return pprof.Labels(labels...)
}
