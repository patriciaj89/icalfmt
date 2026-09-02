// Package icalfmt normalizes iCalendar (RFC 5545) content streams.
package icalfmt

import (
	"bufio"
	"io"
	"strings"
)

// rawLines reads terminator-delimited lines from an underlying reader,
// accepting CRLF, bare LF, and bare CR as line endings (real-world
// exports mix all three). Each call returns one line with its
// terminator stripped.
type rawLines struct {
	br *bufio.Reader
}

func (rl *rawLines) read() (string, error) {
	var b strings.Builder
	sawByte := false
	for {
		c, err := rl.br.ReadByte()
		if err != nil {
			if sawByte {
				return b.String(), nil
			}
			return "", err
		}
		sawByte = true
		switch c {
		case '\n':
			return b.String(), nil
		case '\r':
			if next, peekErr := rl.br.Peek(1); peekErr == nil && next[0] == '\n' {
				rl.br.ReadByte()
			}
			return b.String(), nil
		default:
			b.WriteByte(c)
		}
	}
}

// Unfolder turns a raw iCalendar byte stream into logical content
// lines, reversing the RFC 5545 line-folding rule (a CRLF immediately
// followed by a space or tab is a fold, not a line break).
//
// It only ever holds the current logical line and a one-line lookahead
// in memory, so a calendar with a single huge property (a large
// inline attachment, say) is the only thing that costs memory; the
// rest of the file streams through untouched.
type Unfolder struct {
	rl      *rawLines
	next    string
	hasNext bool
	nextErr error
}

// NewUnfolder wraps r for logical-line reading.
func NewUnfolder(r io.Reader) *Unfolder {
	u := &Unfolder{rl: &rawLines{br: bufio.NewReader(r)}}
	u.advance()
	return u
}

func (u *Unfolder) advance() {
	u.next, u.nextErr = u.rl.read()
	u.hasNext = u.nextErr == nil
}

// Next returns the next logical (unfolded) line and true, or "", false
// once the stream is exhausted. Call Err afterward to distinguish a
// clean end of input from a read failure.
func (u *Unfolder) Next() (string, bool) {
	if !u.hasNext {
		return "", false
	}
	var line strings.Builder
	line.WriteString(u.next)
	u.advance()
	for u.hasNext && len(u.next) > 0 && (u.next[0] == ' ' || u.next[0] == '\t') {
		// RFC 5545: unfolding drops the CRLF and the single
		// whitespace character that follows it, nothing more.
		line.WriteString(u.next[1:])
		u.advance()
	}
	return line.String(), true
}

// Err reports any error other than a clean end of stream.
func (u *Unfolder) Err() error {
	if u.nextErr == io.EOF {
		return nil
	}
	return u.nextErr
}
