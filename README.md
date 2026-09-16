# icalfmt

A formatter for iCalendar (`.ics`, RFC 5545) files that cleans up the
inconsistencies real calendar exports tend to have: mixed line endings,
lines folded at the wrong width or not folded at all, lowercase
property and parameter names, stray blank lines. It rewrites all of
that into a consistent, spec-conformant form without changing any
property or parameter values.

Every export pipeline seems to produce slightly different whitespace.
Outlook, Google Calendar, and hand-rolled scripts all agree on the
RFC 5545 grammar but disagree on the formatting details, and diffing
or hashing two calendars that describe the same events becomes
useless once the formatting differs. Normalizing first makes those
comparisons meaningful again.

## Why streaming matters here

A calendar export can run into the hundreds of thousands of lines (a
synced work calendar going back years, say). `icalfmt` never reads the
whole file into memory: it pulls one logical line at a time off the
input, normalizes it, and writes it straight back out. Memory use is
bounded by the length of the single longest property in the file, not
by the file's total size.

## Usage

As a command:

```sh
go run ./cmd/icalfmt messy.ics > clean.ics

# or from a pipe
cat messy.ics | go run ./cmd/icalfmt > clean.ics
```

As a library:

```go
package main

import (
	"os"

	"icalfmt"
)

func main() {
	f := icalfmt.NewFormatter(os.Stdout)
	if err := f.Format(os.Stdin); err != nil {
		panic(err)
	}
}
```

### Example

Input, with lowercase names, an indented (rather than single-space)
continuation, and mixed line endings:

```
BEGIN:VCALENDAR
version:2.0
prodid:-//Example Corp//Example Calendar//EN
begin:vevent
uid:1234567890@example.com
dtstamp:20260101T120000Z
dtstart;tzid=America/New_York:20260115T090000
summary:Quarterly planning
end:vevent
END:VCALENDAR
```

Output:

```
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example Corp//Example Calendar//EN
BEGIN:VEVENT
UID:1234567890@example.com
DTSTAMP:20260101T120000Z
DTSTART;TZID=America/New_York:20260115T090000
SUMMARY:Quarterly planning
END:VEVENT
END:VCALENDAR
```

Property and parameter names come out upper-cased and every line ends
in CRLF; any line whose content would exceed 75 octets gets re-folded
at a safe boundary (never inside a UTF-8 rune).

## Validation

`Validate` streams a calendar and checks structural correctness: every
`BEGIN` has a matching `END` in the right order, and each component
has the properties RFC 5545 unconditionally requires (`VCALENDAR`
needs `VERSION` and `PRODID`, `VEVENT` needs `UID` and `DTSTAMP`, and
so on). It reports every problem it finds in one pass, joined into a
single error, rather than stopping at the first:

```go
if err := icalfmt.Validate(f); err != nil {
	log.Fatal(err)
}
```

It does not check conditional rules (a `VEVENT` needing `DTSTART`
unless the calendar has a `METHOD`, `VALARM`'s required set varying by
`ACTION`) or value formats — see below.

## What it does not do (yet)

- Value contents are otherwise passed through as-is: RRULEs and the
  value types of X- properties are not normalized. Two exceptions:
  - TEXT-valued standard properties (`SUMMARY`, `DESCRIPTION`, `UID`,
    and the like) have their escaped-newline form canonicalized to
    `\n` (RFC 5545 allows both `\N` and `\n`).
  - DATE-TIME-valued standard properties (`DTSTART`, `DTEND`,
    `DTSTAMP`, `EXDATE`, and the like) have the `T` date/time
    separator and trailing `Z` UTC designator upper-cased, since
    those are case-sensitive per the grammar but some exports get
    them wrong. A bare `DATE` value (no time component) is left
    alone, since it has nothing to normalize.

## Layout

- `unfold.go` — streaming RFC 5545 line unfolding
- `format.go` — normalization and re-folding
- `validate.go` — BEGIN/END nesting and required-property checks
- `cmd/icalfmt` — CLI wrapper

Run the tests with `go test ./...`.
