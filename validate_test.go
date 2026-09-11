package icalfmt

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateWellFormedCalendar(t *testing.T) {
	input := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Example Corp//Example Calendar//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:1234567890@example.com\r\n" +
		"DTSTAMP:20260101T120000Z\r\n" +
		"SUMMARY:Quarterly planning\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	if err := Validate(strings.NewReader(input)); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateCaseInsensitiveNames(t *testing.T) {
	input := "begin:vcalendar\r\n" +
		"version:2.0\r\n" +
		"prodid:-//Example Corp//Example Calendar//EN\r\n" +
		"end:VCALENDAR\r\n"
	if err := Validate(strings.NewReader(input)); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateUnterminatedComponent(t *testing.T) {
	input := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Example Corp//Example Calendar//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:1\r\n" +
		"DTSTAMP:20260101T120000Z\r\n" +
		"END:VCALENDAR\r\n"
	err := Validate(strings.NewReader(input))
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "does not match BEGIN:VEVENT") {
		t.Errorf("Validate() = %v, want mismatch error mentioning BEGIN:VEVENT", err)
	}
}

func TestValidateEndWithoutBegin(t *testing.T) {
	input := "END:VEVENT\r\n"
	err := Validate(strings.NewReader(input))
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() = %v, want *ValidationError", err)
	}
	if verr.Line != 1 || !strings.Contains(verr.Msg, "without matching BEGIN") {
		t.Errorf("got line %d, msg %q", verr.Line, verr.Msg)
	}
}

func TestValidateComponentNeverClosed(t *testing.T) {
	input := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Example Corp//Example Calendar//EN\r\n"
	err := Validate(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "unterminated BEGIN:VCALENDAR") {
		t.Fatalf("Validate() = %v, want unterminated BEGIN:VCALENDAR error", err)
	}
}

func TestValidateMissingRequiredProperty(t *testing.T) {
	input := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:1\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	err := Validate(strings.NewReader(input))
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "VCALENDAR missing required property PRODID") {
		t.Errorf("Validate() = %v, want missing PRODID error", err)
	}
	if !strings.Contains(err.Error(), "VEVENT missing required property DTSTAMP") {
		t.Errorf("Validate() = %v, want missing DTSTAMP error", err)
	}
}

func TestValidateReportsAllErrorsInOnePass(t *testing.T) {
	// Two independent VEVENTs, each missing a different required
	// property: both should show up, not just the first.
	input := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Example Corp//Example Calendar//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:1\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"DTSTAMP:20260101T120000Z\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	err := Validate(strings.NewReader(input))
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	joined := err.Error()
	if !strings.Contains(joined, "missing required property DTSTAMP") {
		t.Errorf("missing DTSTAMP error not found in %q", joined)
	}
	if !strings.Contains(joined, "missing required property UID") {
		t.Errorf("missing UID error not found in %q", joined)
	}
}

func TestValidateIgnoresUnknownComponents(t *testing.T) {
	// X- extension components have no required properties enforced.
	input := "BEGIN:X-CUSTOM\r\n" +
		"X-FOO:bar\r\n" +
		"END:X-CUSTOM\r\n"
	if err := Validate(strings.NewReader(input)); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateBlankLinesAndNonContentLinesIgnored(t *testing.T) {
	input := "BEGIN:VCALENDAR\r\n" +
		"\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Example Corp//Example Calendar//EN\r\n" +
		"garbage with no colon\r\n" +
		"END:VCALENDAR\r\n"
	if err := Validate(strings.NewReader(input)); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}
