# StopWatch

A really, really simple package for timing bits of code. The intention is to
provide a simple, light-weight library for benchmarking specific bits of your
code when need be.

## Example

Pretty straight forward.

```go
package main

import (
	"github.com/bradhe/stopwatch"
	"fmt"
)

func main() {
	start := stopwatch.Start()

	// Do some work.

	watch := stopwatch.Stop(start)
	fmt.Printf("Milliseconds elapsed: %v\n", watch.Milliseconds())
}
```

## Contributing

Really? You want to contribute? Well, okay.

1. Fork and fix/implement in a branch.
1. Make sure tests pass.
1. Make sure you've added new coverage.
1. Submit a PR.
