package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStdinToStdout(t *testing.T) {
	var out bytes.Buffer
	err := run(nil, strings.NewReader("uid:1\r\n"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "UID:1\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunLineLengthFlag(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-l", "12"}, strings.NewReader("SUMMARY:abcdefghij\r\n"), &out)
	if err != nil {
		t.Fatal(err)
	}
	want := "SUMMARY:abcd\r\n efghij\r\n"
	if got := out.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunRejectsTinyLineLength(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-l", "1"}, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected an error for -l 1, got nil")
	}
}

func TestRunInPlaceRequiresFileArgument(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-w"}, strings.NewReader("uid:1\r\n"), &out); err == nil {
		t.Fatal("expected an error for -w with no file argument, got nil")
	}
}

func TestRunFileArgumentToStdout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.ics")
	if err := os.WriteFile(path, []byte("uid:1\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := run([]string{path}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "UID:1\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(unchanged), "uid:1\r\n"; got != want {
		t.Errorf("input file was modified: got %q, want %q", got, want)
	}
}

func TestRunInPlaceRewritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.ics")
	if err := os.WriteFile(path, []byte("uid:1\r\nsummary:hi\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := run([]string{"-w", path}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no stdout output for -w, got %q", out.String())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "UID:1\r\nSUMMARY:hi\r\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
