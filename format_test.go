package icalfmt

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplitContentLine(t *testing.T) {
	cases := []struct {
		line   string
		name   string
		params string
		value  string
		ok     bool
	}{
		{"UID:123", "UID", "", "123", true},
		{
			"DTSTART;TZID=America/New_York:20260115T090000",
			"DTSTART", ";TZID=America/New_York", "20260115T090000", true,
		},
		{
			`X-PROP;PARAM="a:b;c":value`,
			"X-PROP", `;PARAM="a:b;c"`, "value", true,
		},
		{"NOCOLONHERE", "", "", "", false},
		{"", "", "", "", false},
		{":value", "", "", "value", true},
	}
	for _, c := range cases {
		name, params, value, ok := splitContentLine(c.line)
		if name != c.name || params != c.params || value != c.value || ok != c.ok {
			t.Errorf("splitContentLine(%q) = %q, %q, %q, %v; want %q, %q, %q, %v",
				c.line, name, params, value, ok, c.name, c.params, c.value, c.ok)
		}
	}
}

func TestUpperParamNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{";tzid=America/New_York", ";TZID=America/New_York"},
		{";cn=John Doe;role=chair", ";CN=John Doe;ROLE=chair"},
		{`;param="MixedCase:Value"`, `;PARAM="MixedCase:Value"`},
		{`;param="semi;colon:inside";other=val`, `;PARAM="semi;colon:inside";OTHER=val`},
	}
	for _, c := range cases {
		if got := upperParamNames(c.in); got != c.want {
			t.Errorf("upperParamNames(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWriteFoldedUnderLimit(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFolded(&buf, "SUMMARY:short", DefaultLineLength); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "SUMMARY:short\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteFoldedExactlyAtLimit(t *testing.T) {
	line := strings.Repeat("a", DefaultLineLength)
	var buf bytes.Buffer
	if err := writeFolded(&buf, line, DefaultLineLength); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), line+"\r\n"; got != want {
		t.Errorf("got %d bytes, want %d bytes (single unfolded line)", len(got), len(want))
	}
}

func TestWriteFoldedOverLimit(t *testing.T) {
	line := strings.Repeat("a", DefaultLineLength+1)
	var buf bytes.Buffer
	if err := writeFolded(&buf, line, DefaultLineLength); err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("a", DefaultLineLength) + "\r\n" + " a\r\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteFoldedCustomLimit(t *testing.T) {
	line := "SUMMARY:abcdefghij"
	var buf bytes.Buffer
	if err := writeFolded(&buf, line, 10); err != nil {
		t.Fatal(err)
	}
	want := "SUMMARY:ab\r\n cdefghij\r\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteFoldedNeverSplitsARune(t *testing.T) {
	// 74 ASCII bytes, then a two-byte UTF-8 rune landing across the
	// fold boundary, then more content: the fold must back up to keep
	// the rune's two bytes together on the continuation line.
	line := strings.Repeat("a", 74) + "é" + strings.Repeat("b", 10)
	var buf bytes.Buffer
	if err := writeFolded(&buf, line, DefaultLineLength); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	lines := strings.Split(strings.TrimSuffix(got, "\r\n"), "\r\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 folded lines, got %d: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[1], " ") {
		t.Fatalf("continuation line missing leading space: %q", lines[1])
	}
	for i, l := range lines {
		content := l
		if i > 0 {
			content = l[1:]
		}
		if !bytesValidUTF8(content) {
			t.Errorf("line %d is not valid UTF-8: %q", i, l)
		}
	}
	rejoined := lines[0]
	for _, l := range lines[1:] {
		rejoined += l[1:]
	}
	if rejoined != line {
		t.Errorf("rejoined folded output = %q, want %q", rejoined, line)
	}
}

func bytesValidUTF8(s string) bool {
	return strings.ToValidUTF8(s, "�") == s
}

func TestFormatNormalizesCasingFoldingAndLineEndings(t *testing.T) {
	input := "BEGIN:VCALENDAR\n" +
		"version:2.0\r\n" +
		"prodid:-//Example Corp//Example Calendar//EN\r\n" +
		"\r\n" +
		"begin:vevent\r\n" +
		"uid:1234567890@example.com\r\n" +
		"dtstamp:20260101T120000Z\r\n" +
		"dtstart;tzid=America/New_York:20260115T090000\r\n" +
		"summary:Quarterly\r\n  planning\r\n" +
		"end:vevent\r\n" +
		"END:VCALENDAR"

	want := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Example Corp//Example Calendar//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:1234567890@example.com\r\n" +
		"DTSTAMP:20260101T120000Z\r\n" +
		"DTSTART;TZID=America/New_York:20260115T090000\r\n" +
		"SUMMARY:Quarterly planning\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	var buf bytes.Buffer
	if err := NewFormatter(&buf).Format(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != want {
		t.Errorf("Format() output:\n%q\nwant:\n%q", got, want)
	}
}

func TestNormalizeText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"no escapes here", "no escapes here"},
		{`line one\Nline two`, `line one\nline two`},
		{`already\nlowercase`, `already\nlowercase`},
		{`a\\Nb`, `a\\Nb`}, // escaped backslash followed by a literal N, not an escape
		{`\N\N`, `\n\n`},
		{`comma\, semi\; back\\ newline\N end`, `comma\, semi\; back\\ newline\n end`},
	}
	for _, c := range cases {
		if got := normalizeText(c.in); got != c.want {
			t.Errorf("normalizeText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatNormalizesTextEscapesOnKnownProperties(t *testing.T) {
	input := "BEGIN:VEVENT\r\n" +
		`DESCRIPTION:Line one\NLine two` + "\r\n" +
		`X-CUSTOM:Line one\NLine two` + "\r\n" +
		"END:VEVENT\r\n"
	want := "BEGIN:VEVENT\r\n" +
		`DESCRIPTION:Line one\nLine two` + "\r\n" +
		`X-CUSTOM:Line one\NLine two` + "\r\n" +
		"END:VEVENT\r\n"

	var buf bytes.Buffer
	if err := NewFormatter(&buf).Format(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != want {
		t.Errorf("Format() output:\n%q\nwant:\n%q", got, want)
	}
}

func TestNormalizeDateTime(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"20260115", "20260115"}, // bare DATE: nothing to normalize
		{"20260115T090000", "20260115T090000"},
		{"20260115t090000", "20260115T090000"},
		{"20260115T090000Z", "20260115T090000Z"},
		{"20260115t090000z", "20260115T090000Z"},
		{"20260115T090000z", "20260115T090000Z"},
		{ // EXDATE/RDATE comma list
			"20260115t090000z,20260116T100000",
			"20260115T090000Z,20260116T100000",
		},
		{ // RDATE PERIOD value
			"20260115t090000z/20260116t100000z",
			"20260115T090000Z/20260116T100000Z",
		},
		{"-PT15M", "-PT15M"},                     // TRIGGER duration: no digit run to match
		{"2026011", "2026011"},                   // too short to be a date
		{"202601150t090000", "202601150t090000"}, // 9-digit run, not an 8-digit date
	}
	for _, c := range cases {
		if got := normalizeDateTime(c.in); got != c.want {
			t.Errorf("normalizeDateTime(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatNormalizesDateTimeOnKnownProperties(t *testing.T) {
	input := "BEGIN:VEVENT\r\n" +
		"DTSTART:20260115t090000z\r\n" +
		"DTEND;VALUE=DATE:20260116\r\n" +
		"EXDATE:20260115t090000z,20260122t090000z\r\n" +
		"X-CUSTOM-DATE:20260115t090000z\r\n" +
		"END:VEVENT\r\n"
	want := "BEGIN:VEVENT\r\n" +
		"DTSTART:20260115T090000Z\r\n" +
		"DTEND;VALUE=DATE:20260116\r\n" +
		"EXDATE:20260115T090000Z,20260122T090000Z\r\n" +
		"X-CUSTOM-DATE:20260115t090000z\r\n" +
		"END:VEVENT\r\n"

	var buf bytes.Buffer
	if err := NewFormatter(&buf).Format(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != want {
		t.Errorf("Format() output:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatPassesThroughNonContentLines(t *testing.T) {
	input := "not a content line at all\r\nUID:1\r\n"
	want := "not a content line at all\r\nUID:1\r\n"

	var buf bytes.Buffer
	if err := NewFormatter(&buf).Format(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != want {
		t.Errorf("Format() output = %q, want %q", got, want)
	}
}
