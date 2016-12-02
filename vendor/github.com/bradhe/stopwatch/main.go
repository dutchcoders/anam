package stopwatch

import (
	"time"
)

func Start() time.Time {
	return time.Now()
}

func Stop(start time.Time) *StopWatch {
	watch := StopWatch{start: start, stop: time.Now()}
	return &watch
}
