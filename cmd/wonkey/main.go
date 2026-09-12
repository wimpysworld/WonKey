package main

import (
	"os"
	"wonkey/internal/xfkey"
)

func main() {
	if err := xfkey.RunIO(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(xfkey.ExitCode(err))
	}
}
