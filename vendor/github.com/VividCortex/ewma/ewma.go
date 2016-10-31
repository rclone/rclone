// Package ewma implements exponentially weighted moving averages.
package ewma

// Copyright (c) 2013 VividCortex, Inc. All rights reserved.
// Please see the LICENSE file for applicable license terms.

const (
	// By default, we average over a one-minute period, which means the average
	// age of the metrics in the period is 30 seconds.
	AVG_METRIC_AGE float64 = 30.0

	// The formula for computing the decay factor from the average age comes
	// from "Production and Operations Analysis" by Steven Nahmias.
	DECAY float64 = 2 / (float64(AVG_METRIC_AGE) + 1)

	// For best results, the moving average should not be initialized to the
	// samples it sees immediately. The book "Production and Operations
	// Analysis" by Steven Nahmias suggests initializing the moving average to
	// the mean of the first 10 samples. Until the VariableEwma has seen this
	// many samples, it is not "ready" to be queried for the value of the
	// moving average. This adds some memory cost.
	WARMUP_SAMPLES uint8 = 10
)

// MovingAverage is the interface that computes a moving average over a time-
// series stream of numbers. The average may be over a window or exponentially
// decaying.
type MovingAverage interface {
	Add(float64)
	Value() float64
	Set(float64)
}

// NewMovingAverage constructs a MovingAverage that computes an average with the
// desired characteristics in the moving window or exponential decay. If no
// age is given, it constructs a default exponentially weighted implementation
// that consumes minimal memory. The age is related to the decay factor alpha
// by the formula given for the DECAY constant. It signifies the average age
// of the samples as time goes to infinity.
func NewMovingAverage(age ...float64) MovingAverage {
	if len(age) == 0 || age[0] == AVG_METRIC_AGE {
		return new(SimpleEWMA)
	}
	return &VariableEWMA{
		decay: 2 / (age[0] + 1),
	}
}

// A SimpleEWMA represents the exponentially weighted moving average of a
// series of numbers. It WILL have different behavior than the VariableEWMA
// for multiple reasons. It has no warm-up period and it uses a constant
// decay.  These properties let it use less memory.  It will also behave
// differently when it's equal to zero, which is assumed to mean
// uninitialized, so if a value is likely to actually become zero over time,
// then any non-zero value will cause a sharp jump instead of a small change.
// However, note that this takes a long time, and the value may just
// decays to a stable value that's close to zero, but which won't be mistaken
// for uninitialized. See http://play.golang.org/p/litxBDr_RC for example.
type SimpleEWMA struct {
	// The current value of the average. After adding with Add(), this is
	// updated to reflect the average of all values seen thus far.
	value float64
}

// Add adds a value to the series and updates the moving average.
func (e *SimpleEWMA) Add(value float64) {
	if e.value == 0 { // this is a proxy for "uninitialized"
		e.value = value
	} else {
		e.value = (value * DECAY) + (e.value * (1 - DECAY))
	}
}

// Value returns the current value of the moving average.
func (e *SimpleEWMA) Value() float64 {
	return e.value
}

// Set sets the EWMA's value.
func (e *SimpleEWMA) Set(value float64) {
	e.value = value
}

// VariableEWMA represents the exponentially weighted moving average of a series of
// numbers. Unlike SimpleEWMA, it supports a custom age, and thus uses more memory.
type VariableEWMA struct {
	// The multiplier factor by which the previous samples decay.
	decay float64
	// The current value of the average.
	value float64
	// The number of samples added to this instance.
	count uint8
}

// Add adds a value to the series and updates the moving average.
func (e *VariableEWMA) Add(value float64) {
	switch {
	case e.count < WARMUP_SAMPLES:
		e.count++
		e.value += value
	case e.count == WARMUP_SAMPLES:
		e.count++
		e.value = e.value / float64(WARMUP_SAMPLES)
		e.value = (value * e.decay) + (e.value * (1 - e.decay))
	default:
		e.value = (value * e.decay) + (e.value * (1 - e.decay))
	}
}

// Value returns the current value of the average, or 0.0 if the series hasn't
// warmed up yet.
func (e *VariableEWMA) Value() float64 {
	if e.count <= WARMUP_SAMPLES {
		return 0.0
	}

	return e.value
}

// Set sets the EWMA's value.
func (e *VariableEWMA) Set(value float64) {
	e.value = value
	if e.count <= WARMUP_SAMPLES {
		e.count = WARMUP_SAMPLES + 1
	}
}
