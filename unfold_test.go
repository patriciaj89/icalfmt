package icalfmt

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func unfoldAll(t *testing.T, input string) []string {
	t.Helper()
	u := NewUnfolder(strings.NewReader(input))
	var lines []string
	for {
		line, ok := u.Next()
		if !ok {
			break
		}
		lines = append(lines, line)
	}
	if err := u.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	return lines
}

func TestUnfolderNoFolding(t *testing.T) {
	got := unfoldAll(t, "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")
	want := []string{"BEGIN:VCALENDAR", "END:VCALENDAR"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUnfolderSpaceContinuation(t *testing.T) {
	// Unfolding removes the CRLF and exactly the one whitespace
	// character that follows it; it does not insert anything back.
	got := unfoldAll(t, "SUMMARY:AB\r\n CD\r\n")
	want := []string{"SUMMARY:ABCD"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnfolderTabContinuation(t *testing.T) {
	// The single whitespace character that introduced the fold is
	// dropped regardless of whether it was a space or a tab.
	got := unfoldAll(t, "SUMMARY:Long\r\n\ttitle\r\n")
	want := "SUMMARY:Longtitle"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnfolderMultipleContinuations(t *testing.T) {
	got := unfoldAll(t, "DESCRIPTION:A\r\n B\r\n\tC\r\n D\r\n")
	want := "DESCRIPTION:ABCD"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnfolderOnlyDropsOneWhitespaceCharacter(t *testing.T) {
	// A continuation line with two leading spaces keeps one of them:
	// only the fold-introducing whitespace is removed by unfolding.
	got := unfoldAll(t, "SUMMARY:a\r\n  b\r\n")
	want := "SUMMARY:a b"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnfolderMixedLineEndings(t *testing.T) {
	got := unfoldAll(t, "BEGIN:VEVENT\nUID:1\r\nSUMMARY:x\rEND:VEVENT\r\n")
	want := []string{"BEGIN:VEVENT", "UID:1", "SUMMARY:x", "END:VEVENT"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUnfolderNoTrailingTerminator(t *testing.T) {
	got := unfoldAll(t, "BEGIN:VCALENDAR\r\nEND:VCALENDAR")
	want := []string{"BEGIN:VCALENDAR", "END:VCALENDAR"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUnfolderFoldAtVeryEndOfStream(t *testing.T) {
	// A continuation line with no terminator at all is still a fold.
	got := unfoldAll(t, "SUMMARY:a\r\n b")
	want := "SUMMARY:ab"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnfolderEmptyInput(t *testing.T) {
	got := unfoldAll(t, "")
	if len(got) != 0 {
		t.Fatalf("got %q, want no lines", got)
	}
}

type errReader struct {
	data []byte
	err  error
}

func (r *errReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}

func TestUnfolderPropagatesReadError(t *testing.T) {
	wantErr := errors.New("boom")
	u := NewUnfolder(&errReader{data: []byte("SUMMARY:x\r\n"), err: wantErr})

	line, ok := u.Next()
	if !ok || line != "SUMMARY:x" {
		t.Fatalf("Next() = %q, %v, want %q, true", line, ok, "SUMMARY:x")
	}

	_, ok = u.Next()
	if ok {
		t.Fatalf("Next() ok = true after read error, want false")
	}
	if err := u.Err(); !errors.Is(err, wantErr) {
		t.Fatalf("Err() = %v, want %v", err, wantErr)
	}
}

func TestUnfolderErrIsNilOnCleanEOF(t *testing.T) {
	u := NewUnfolder(strings.NewReader("UID:1\r\n"))
	for {
		if _, ok := u.Next(); !ok {
			break
		}
	}
	if err := u.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if !errors.Is(u.nextErr, io.EOF) {
		t.Fatalf("nextErr = %v, want io.EOF", u.nextErr)
	}
}
