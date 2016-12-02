package stopwatch

import (
	"time"
)

type StopWatch struct {
	start, stop time.Time
}

func (self *StopWatch) Milliseconds() uint32 {
	return uint32(self.stop.Sub(self.start) / time.Millisecond)
}
