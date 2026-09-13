package icalfmt

import (
	"bufio"
	"io"
	"strings"
)

// maxOctets is the RFC 5545 recommended maximum content line length,
// not counting the CRLF terminator.
const maxOctets = 75

// Formatter reads an iCalendar stream and writes it back out in a
// normalized form: consistent CRLF line endings, upper-cased property
// and parameter names, and lines re-folded to the RFC 5545 length
// limit. Property and parameter values are passed through untouched.
type Formatter struct {
	w *bufio.Writer
}

// NewFormatter wraps w for normalized output.
func NewFormatter(w io.Writer) *Formatter {
	return &Formatter{w: bufio.NewWriter(w)}
}

// Format streams r through the formatter, writing normalized content
// to the Formatter's underlying writer and flushing before returning.
// It processes one logical line at a time, so the size of r does not
// bound the memory this uses.
func (f *Formatter) Format(r io.Reader) error {
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
			if err := writeFolded(f.w, line); err != nil {
				return err
			}
			continue
		}
		nameUpper := strings.ToUpper(name)
		if textValueProperties[nameUpper] {
			value = normalizeText(value)
		}
		normalized := nameUpper + upperParamNames(params) + ":" + value
		if err := writeFolded(f.w, normalized); err != nil {
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

func asciiUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

// writeFolded writes line to w as one or more RFC 5545 content lines:
// CRLF-terminated, folded so no line exceeds maxOctets octets, with
// continuation lines prefixed by a single space. Folding never splits
// a UTF-8 rune.
func writeFolded(w io.Writer, line string) error {
	b := []byte(line)
	first := true
	for {
		limit := maxOctets
		if !first {
			limit-- // the continuation's leading space counts
		}
		if len(b) <= limit {
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

		n := limit
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
