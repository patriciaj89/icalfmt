// Command icalfmt normalizes an iCalendar file and prints the result
// to stdout. With no arguments it reads from stdin, so it composes
// with a pipeline.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"icalfmt"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "icalfmt:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("icalfmt", flag.ContinueOnError)
	lineLength := fs.Int("l", icalfmt.DefaultLineLength,
		"maximum content line length in octets before folding")
	inPlace := fs.Bool("w", false,
		"write the result back to the input file instead of stdout (requires a file argument)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// A fold needs at least one octet on the first line and one on
	// each continuation past its leading space; anything smaller
	// can't converge.
	if *lineLength < 2 {
		return fmt.Errorf("-l must be at least 2, got %d", *lineLength)
	}

	fargs := fs.Args()
	if *inPlace && len(fargs) == 0 {
		return fmt.Errorf("-w requires a file argument")
	}

	if len(fargs) == 0 {
		f := icalfmt.NewFormatter(stdout)
		f.LineLength = *lineLength
		return f.Format(stdin)
	}

	path := fargs[0]
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	if !*inPlace {
		f := icalfmt.NewFormatter(stdout)
		f.LineLength = *lineLength
		return f.Format(in)
	}

	return formatInPlace(path, in, *lineLength)
}

// formatInPlace normalizes in (the already-opened file at path) into a
// temporary file in the same directory and renames it over path, so a
// crash or write failure partway through never leaves the original
// truncated or half-rewritten.
func formatInPlace(path string, in *os.File, lineLength int) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".icalfmt-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	f := icalfmt.NewFormatter(tmp)
	f.LineLength = lineLength
	if err := f.Format(in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
