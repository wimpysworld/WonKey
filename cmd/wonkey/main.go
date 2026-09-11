package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"wonkey/internal/xfkey"
)

func main() {
	if err := xfkey.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "wonkey:", err)
		os.Exit(1)
	}
}
