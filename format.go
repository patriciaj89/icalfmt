package icalfmt

import (
	"bufio"
	"io"
	"strings"
)

// DefaultLineLength is the RFC 5545 recommended maximum content line
// length, not counting the CRLF terminator.
const DefaultLineLength = 75

// Formatter reads an iCalendar stream and writes it back out in a
// normalized form: consistent CRLF line endings, upper-cased property
// and parameter names, and lines re-folded to LineLength. Property and
// parameter values are passed through untouched.
type Formatter struct {
	// LineLength is the maximum octet count of a folded line,
	// excluding the CRLF terminator. NewFormatter sets it to
	// DefaultLineLength; callers may change it before calling
	// Format.
	LineLength int

	w *bufio.Writer
}

// NewFormatter wraps w for normalized output.
func NewFormatter(w io.Writer) *Formatter {
	return &Formatter{w: bufio.NewWriter(w), LineLength: DefaultLineLength}
}

// Format streams r through the formatter, writing normalized content
// to the Formatter's underlying writer and flushing before returning.
// It processes one logical line at a time, so the size of r does not
// bound the memory this uses.
func (f *Formatter) Format(r io.Reader) error {
	limit := f.LineLength
	if limit <= 0 {
		limit = DefaultLineLength
	}
	u := NewUnfolder(r)
	for {
		line, ok := u.Next()
		if !ok {
			break
		}
		line = strings.TrimRight(line, " \t")
		if line == "" {
			// Stray blank lines show up in hand-edited or
			// badly-exported calendars; RFC 5545 has no use
			// for them between content lines.
			continue
		}
		name, params, value, isContentLine := splitContentLine(line)
		if !isContentLine {
			// No unquoted colon at all: not a valid content
			// line. Pass it through rather than silently
			// dropping data we don't understand.
			if err := writeFolded(f.w, line, limit); err != nil {
				return err
			}
			continue
		}
		nameUpper := strings.ToUpper(name)
		switch {
		case textValueProperties[nameUpper]:
			value = normalizeText(value)
		case dateTimeValueProperties[nameUpper]:
			value = normalizeDateTime(value)
		}
		normalized := nameUpper + upperParamNames(params) + ":" + value
		if err := writeFolded(f.w, normalized, limit); err != nil {
			return err
		}
	}
	if err := u.Err(); err != nil {
		return err
	}
	return f.w.Flush()
}

// splitContentLine splits a logical content line into its property
// name, raw parameter section (including the leading ";", or empty),
// and value, per the RFC 5545 ABNF:
//
//	contentline = name *(";" param) ":" value
//
// The scan tracks quoted-string state because a quoted parameter value
// may itself contain ":" or ";".
func splitContentLine(line string) (name, params, value string, ok bool) {
	nameEnd := -1
	inQuotes := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQuotes = !inQuotes
		case ';', ':':
			if !inQuotes {
				nameEnd = i
			}
		}
		if nameEnd != -1 {
			break
		}
	}
	if nameEnd == -1 {
		return "", "", "", false
	}

	rest := line[nameEnd:]
	inQuotes = false
	colon := -1
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '"':
			inQuotes = !inQuotes
		case ':':
			if !inQuotes {
				colon = i
			}
		}
		if colon != -1 {
			break
		}
	}
	if colon == -1 {
		return "", "", "", false
	}

	return line[:nameEnd], rest[:colon], rest[colon+1:], true
}

// upperParamNames upper-cases each parameter name in a ";NAME=value"
// section while leaving "=value" (including anything inside quotes)
// untouched.
func upperParamNames(params string) string {
	var out strings.Builder
	out.Grow(len(params))
	inQuotes := false
	inName := false
	for i := 0; i < len(params); i++ {
		c := params[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
			out.WriteByte(c)
		case inQuotes:
			out.WriteByte(c)
		case c == ';':
			inName = true
			out.WriteByte(c)
		case c == '=':
			inName = false
			out.WriteByte(c)
		case inName:
			out.WriteByte(asciiUpper(c))
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// textValueProperties lists the standard RFC 5545 properties whose
// value type is TEXT (or a comma-separated list of TEXT), where the
// backslash-escaping rules in section 3.3.11 apply. Properties not
// listed here (including X- properties, whose value type is only
// known via an optional VALUE parameter) are left untouched.
var textValueProperties = map[string]bool{
	"ACTION":      true,
	"CATEGORIES":  true,
	"CLASS":       true,
	"COMMENT":     true,
	"CONTACT":     true,
	"DESCRIPTION": true,
	"LOCATION":    true,
	"METHOD":      true,
	"PRODID":      true,
	"RELATED-TO":  true,
	"RESOURCES":   true,
	"STATUS":      true,
	"SUMMARY":     true,
	"TRANSP":      true,
	"TZID":        true,
	"TZNAME":      true,
	"UID":         true,
}

// normalizeText canonicalizes the escaped-newline form of a TEXT value.
// RFC 5545 section 3.3.11 allows a literal newline inside TEXT to be
// escaped as either "\N" or "\n"; exports disagree on which they use,
// which defeats byte-for-byte comparison between otherwise identical
// calendars. This rewrites "\N" to "\n" wherever it appears as an
// escape (i.e. preceded by an even number of backslashes) and leaves
// every other character, including other escape sequences, untouched.
func normalizeText(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	escaped := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		if escaped {
			if c == 'N' {
				out.WriteByte('n')
			} else {
				out.WriteByte(c)
			}
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
		}
		out.WriteByte(c)
	}
	return out.String()
}

// dateTimeValueProperties lists the standard RFC 5545 properties whose
// value type is DATE-TIME by default (section 3.8.2, 3.8.4, 3.8.5, 3.8.6,
// 3.8.7). A DTSTART, DTEND, DUE, RECURRENCE-ID, EXDATE, or RDATE can be
// switched to DATE via a VALUE parameter, and TRIGGER can be switched to
// DURATION, but normalizeDateTime only rewrites substrings that already
// look like a DATE-TIME, so running it on a DATE or DURATION value is a
// no-op rather than a misfire.
var dateTimeValueProperties = map[string]bool{
	"CREATED":       true,
	"DTEND":         true,
	"DTSTAMP":       true,
	"DTSTART":       true,
	"DUE":           true,
	"EXDATE":        true,
	"LAST-MODIFIED": true,
	"RDATE":         true,
	"RECURRENCE-ID": true,
	"TRIGGER":       true,
}

// normalizeDateTime upper-cases the "T" date/time separator and trailing
// "Z" UTC designator in every DATE-TIME-shaped substring of value:
// 8 digits, "T" or "t", 6 digits, and an optional "Z" or "z". This covers
// EXDATE/RDATE comma-lists and RDATE PERIOD values too, since it scans
// for the shape rather than requiring the whole value to match it.
// Anything that isn't shaped like a DATE-TIME (a bare DATE, a DURATION
// on TRIGGER) is left untouched.
func normalizeDateTime(value string) string {
	b := []byte(value)
	n := len(b)
	for i := 0; i+8 <= n; {
		if !isDigitRun(b[i : i+8]) {
			i++
			continue
		}
		if i > 0 && isDigit(b[i-1]) {
			// Part of a longer digit run, not an 8-digit date.
			i++
			continue
		}
		t := i + 8
		if t >= n || (b[t] != 't' && b[t] != 'T') {
			i = t
			continue
		}
		timeEnd := t + 7 // one past the 6 time digits
		if timeEnd > n || !isDigitRun(b[t+1:timeEnd]) {
			i = t + 1
			continue
		}
		if timeEnd < n && isDigit(b[timeEnd]) {
			// More than 6 digits after T: not a valid time.
			i = timeEnd
			continue
		}
		b[t] = 'T'
		if timeEnd < n && (b[timeEnd] == 'z' || b[timeEnd] == 'Z') {
			b[timeEnd] = 'Z'
			timeEnd++
		}
		i = timeEnd
	}
	return string(b)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isDigitRun(b []byte) bool {
	for _, c := range b {
		if !isDigit(c) {
			return false
		}
	}
	return true
}

func asciiUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

// writeFolded writes line to w as one or more RFC 5545 content lines:
// CRLF-terminated, folded so no line exceeds limit octets, with
// continuation lines prefixed by a single space. Folding never splits
// a UTF-8 rune.
func writeFolded(w io.Writer, line string, limit int) error {
	b := []byte(line)
	first := true
	for {
		lim := limit
		if !first {
			lim-- // the continuation's leading space counts
		}
		if len(b) <= lim {
			if !first {
				if _, err := w.Write([]byte{' '}); err != nil {
					return err
				}
			}
			if _, err := w.Write(b); err != nil {
				return err
			}
			_, err := w.Write([]byte("\r\n"))
			return err
		}

		n := lim
		for n > 0 && isUTF8Continuation(b[n]) {
			n--
		}
		if !first {
			if _, err := w.Write([]byte{' '}); err != nil {
				return err
			}
		}
		if _, err := w.Write(b[:n]); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\r\n")); err != nil {
			return err
		}
		b = b[n:]
		first = false
	}
}

func isUTF8Continuation(c byte) bool {
	return c&0xC0 == 0x80
}
