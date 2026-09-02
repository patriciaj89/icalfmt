// Command icalfmt normalizes an iCalendar file and prints the result
// to stdout. With no arguments it reads from stdin, so it composes
// with a pipeline.
package main

import (
	"fmt"
	"io"
	"os"

	"icalfmt"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "icalfmt:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	in := stdin
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	return icalfmt.NewFormatter(stdout).Format(in)
}
