package report

import (
	"testing"
	"time"
)

func TestResolveDateRangeFromTo(t *testing.T) {
	now := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.UTC)
	from, to, err := ResolveDateRange(PeriodOptions{FromDate: "2026-02-01", ToDate: "2026-02-10"}, now)
	if err != nil {
		t.Fatalf("resolve explicit range: %v", err)
	}
	if from.Format(time.RFC3339) != "2026-02-01T00:00:00Z" {
		t.Fatalf("unexpected from: %s", from.Format(time.RFC3339))
	}
	if to.Format(time.RFC3339) != "2026-02-10T23:59:59Z" {
		t.Fatalf("unexpected to: %s", to.Format(time.RFC3339))
	}
}

func TestResolveDateRangeExplicitPrecedence(t *testing.T) {
	now := time.Date(2026, time.March, 5, 10, 0, 0, 0, time.UTC)
	from, to, err := ResolveDateRange(PeriodOptions{
		FromDate: "2026-01-01",
		ToDate:   "2026-01-31",
		LastDays: 7,
	}, now)
	if err != nil {
		t.Fatalf("resolve explicit precedence: %v", err)
	}
	if from.Format("2006-01-02") != "2026-01-01" || to.Format("2006-01-02") != "2026-01-31" {
		t.Fatalf("explicit dates should take precedence, got %s -> %s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
}

func TestResolveDateRangeLastDays(t *testing.T) {
	now := time.Date(2026, time.March, 5, 14, 30, 0, 0, time.UTC)
	from, to, err := ResolveDateRange(PeriodOptions{LastDays: 7}, now)
	if err != nil {
		t.Fatalf("resolve last-days: %v", err)
	}
	if from.Format(time.RFC3339) != "2026-02-27T00:00:00Z" {
		t.Fatalf("unexpected from: %s", from.Format(time.RFC3339))
	}
	if to.Format(time.RFC3339) != "2026-03-05T23:59:59Z" {
		t.Fatalf("unexpected to: %s", to.Format(time.RFC3339))
	}
}

func TestResolveDateRangeLastWeeks(t *testing.T) {
	now := time.Date(2026, time.March, 5, 14, 30, 0, 0, time.UTC)
	from, to, err := ResolveDateRange(PeriodOptions{LastWeeks: 2}, now)
	if err != nil {
		t.Fatalf("resolve last-weeks: %v", err)
	}
	if from.Format(time.RFC3339) != "2026-02-20T00:00:00Z" {
		t.Fatalf("unexpected from: %s", from.Format(time.RFC3339))
	}
	if to.Format(time.RFC3339) != "2026-03-05T23:59:59Z" {
		t.Fatalf("unexpected to: %s", to.Format(time.RFC3339))
	}
}

func TestResolveDateRangeThisYear(t *testing.T) {
	now := time.Date(2026, time.June, 2, 8, 0, 0, 0, time.UTC)
	from, to, err := ResolveDateRange(PeriodOptions{ThisYear: true}, now)
	if err != nil {
		t.Fatalf("resolve this-year: %v", err)
	}
	if from.Format(time.RFC3339) != "2026-01-01T00:00:00Z" {
		t.Fatalf("unexpected from: %s", from.Format(time.RFC3339))
	}
	if to.Format(time.RFC3339) != "2026-06-02T23:59:59Z" {
		t.Fatalf("unexpected to: %s", to.Format(time.RFC3339))
	}
}

func TestResolveDateRangeValidationErrors(t *testing.T) {
	now := time.Date(2026, time.January, 10, 9, 0, 0, 0, time.UTC)
	cases := []PeriodOptions{
		{},
		{FromDate: "2026-01-01"},
		{LastDays: 0, LastWeeks: 1, ThisYear: true},
		{LastDays: -1},
		{LastWeeks: -3},
	}
	for _, tc := range cases {
		if _, _, err := ResolveDateRange(tc, now); err == nil {
			t.Fatalf("expected error for %#v", tc)
		}
	}
}

func TestParseRelativePeriod(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected PeriodOptions
	}{
		{name: "last days", input: []string{"last", "7", "days"}, expected: PeriodOptions{LastDays: 7}},
		{name: "last weeks", input: []string{"last", "2", "weeks"}, expected: PeriodOptions{LastWeeks: 2}},
		{name: "this year", input: []string{"this", "year"}, expected: PeriodOptions{ThisYear: true}},
	}

	for _, tt := range tests {
		got, err := ParseRelativePeriod(tt.input)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got != tt.expected {
			t.Fatalf("%s: got %#v want %#v", tt.name, got, tt.expected)
		}
	}
}

func TestParseRelativePeriodErrors(t *testing.T) {
	cases := [][]string{
		{"last", "0", "days"},
		{"last", "a", "weeks"},
		{"last", "3", "months"},
		{"this"},
	}

	for _, in := range cases {
		if _, err := ParseRelativePeriod(in); err == nil {
			t.Fatalf("expected error for %v", in)
		}
	}
}
