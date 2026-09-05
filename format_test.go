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
	if err := writeFolded(&buf, "SUMMARY:short"); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "SUMMARY:short\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteFoldedExactlyAtLimit(t *testing.T) {
	line := strings.Repeat("a", maxOctets)
	var buf bytes.Buffer
	if err := writeFolded(&buf, line); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), line+"\r\n"; got != want {
		t.Errorf("got %d bytes, want %d bytes (single unfolded line)", len(got), len(want))
	}
}

func TestWriteFoldedOverLimit(t *testing.T) {
	line := strings.Repeat("a", maxOctets+1)
	var buf bytes.Buffer
	if err := writeFolded(&buf, line); err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("a", maxOctets) + "\r\n" + " a\r\n"
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
	if err := writeFolded(&buf, line); err != nil {
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
