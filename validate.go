package icalfmt

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// requiredProperties lists the properties RFC 5545 requires directly
// inside each component type. Components not listed here (including
// vendor X- components) have no required properties enforced.
//
// This intentionally does not encode the conditional rules RFC 5545
// also specifies, such as VEVENT needing DTSTART unless the enclosing
// VCALENDAR has a METHOD property, or VALARM's required set varying by
// ACTION. Catching the unconditional cases still catches most
// malformed exports.
var requiredProperties = map[string][]string{
	"VCALENDAR": {"PRODID", "VERSION"},
	"VEVENT":    {"UID", "DTSTAMP"},
	"VTODO":     {"UID", "DTSTAMP"},
	"VJOURNAL":  {"UID", "DTSTAMP"},
	"VFREEBUSY": {"UID", "DTSTAMP"},
	"VTIMEZONE": {"TZID"},
	"VALARM":    {"ACTION", "TRIGGER"},
}

// componentFrame tracks one open BEGIN...END component while walking
// the stream: which properties it has seen directly (not inside a
// nested component), and the logical line its BEGIN appeared on, for
// error messages.
type componentFrame struct {
	name string
	line int
	seen map[string]bool
}

// ValidationError describes one structural problem found by Validate,
// anchored to the logical (unfolded) line it was detected on.
type ValidationError struct {
	Line int
	Msg  string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// Validate streams r and checks BEGIN/END nesting balance and the
// presence of RFC 5545's unconditionally required properties in each
// component. It does not check value formats or property cardinality.
//
// All problems found are returned together via errors.Join, so a
// single pass reports everything wrong with a calendar rather than
// just the first issue. It returns nil if the stream is structurally
// valid.
func Validate(r io.Reader) error {
	u := NewUnfolder(r)
	var stack []*componentFrame
	var errs []error
	lineNum := 0

	for {
		line, ok := u.Next()
		if !ok {
			break
		}
		lineNum++
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		name, _, value, isContentLine := splitContentLine(line)
		if !isContentLine {
			continue
		}
		nameUpper := strings.ToUpper(name)

		switch nameUpper {
		case "BEGIN":
			stack = append(stack, &componentFrame{
				name: strings.ToUpper(value),
				line: lineNum,
				seen: make(map[string]bool),
			})
		case "END":
			valUpper := strings.ToUpper(value)
			if len(stack) == 0 {
				errs = append(errs, &ValidationError{
					Line: lineNum,
					Msg:  fmt.Sprintf("END:%s without matching BEGIN", valUpper),
				})
				continue
			}
			top := stack[len(stack)-1]
			if top.name != valUpper {
				errs = append(errs, &ValidationError{
					Line: lineNum,
					Msg:  fmt.Sprintf("END:%s does not match BEGIN:%s at line %d", valUpper, top.name, top.line),
				})
			} else {
				for _, req := range requiredProperties[top.name] {
					if !top.seen[req] {
						errs = append(errs, &ValidationError{
							Line: top.line,
							Msg:  fmt.Sprintf("%s missing required property %s", top.name, req),
						})
					}
				}
			}
			stack = stack[:len(stack)-1]
		default:
			if len(stack) > 0 {
				stack[len(stack)-1].seen[nameUpper] = true
			}
		}
	}
	if err := u.Err(); err != nil {
		return err
	}

	for _, frame := range stack {
		errs = append(errs, &ValidationError{
			Line: frame.line,
			Msg:  fmt.Sprintf("unterminated BEGIN:%s", frame.name),
		})
	}

	return errors.Join(errs...)
}
