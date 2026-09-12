package xfkey

import (
	"io"
	"strings"
)

func printHumanHelp(out io.Writer, text string) error {
	if h := newHuman(out); h.colour {
		heading, rest, _ := strings.Cut(text, "\n")
		text = h.style("35", heading) + "\n" + rest
	}
	_, err := io.WriteString(out, text)
	return err
}
