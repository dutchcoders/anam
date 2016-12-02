package main

import (
	cmd "github.com/dutchcoders/anam/cmd"
)

func main() {
	app := cmd.New()
	app.RunAndExitOnError()
}
