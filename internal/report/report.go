package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func HumanDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02dh %02dm %02ds", h, m, s)
}

func ParseDateRange(from, to string) (time.Time, time.Time, error) {
	fromTime, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --from date: %w", err)
	}
	toTime, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --to date: %w", err)
	}
	toTime = toTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	if toTime.Before(fromTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to date must be on or after --from")
	}
	return fromTime.UTC(), toTime.UTC(), nil
}

type PeriodOptions struct {
	FromDate  string
	ToDate    string
	LastDays  int
	LastWeeks int
	ThisYear  bool
}

func ParseRelativePeriod(parts []string) (PeriodOptions, error) {
	normalized := make([]string, 0, len(parts))
	for _, p := range parts {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(p)))
	}

	switch {
	case len(normalized) == 2 && normalized[0] == "this" && normalized[1] == "year":
		return PeriodOptions{ThisYear: true}, nil
	case len(normalized) == 3 && normalized[0] == "last":
		n, err := strconv.Atoi(normalized[1])
		if err != nil || n <= 0 {
			return PeriodOptions{}, fmt.Errorf("invalid period count %q: must be a positive integer", parts[1])
		}
		switch normalized[2] {
		case "day", "days":
			return PeriodOptions{LastDays: n}, nil
		case "week", "weeks":
			return PeriodOptions{LastWeeks: n}, nil
		default:
			return PeriodOptions{}, fmt.Errorf("invalid period unit %q: use days or weeks", parts[2])
		}
	default:
		return PeriodOptions{}, fmt.Errorf("invalid period format: use YYYY-MM-DD YYYY-MM-DD, last N days, last N weeks, or this year")
	}
}

func ResolveDateRange(opts PeriodOptions, now time.Time) (time.Time, time.Time, error) {
	explicitProvided := opts.FromDate != "" || opts.ToDate != ""
	if explicitProvided {
		if opts.FromDate == "" || opts.ToDate == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("--from and --to must be provided together")
		}
		return ParseDateRange(opts.FromDate, opts.ToDate)
	}

	relativeCount := 0
	if opts.LastDays != 0 {
		relativeCount++
	}
	if opts.LastWeeks != 0 {
		relativeCount++
	}
	if opts.ThisYear {
		relativeCount++
	}
	if relativeCount == 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("provide --from and --to, --last-days, --last-weeks, or --this-year")
	}
	if relativeCount > 1 {
		return time.Time{}, time.Time{}, fmt.Errorf("use only one relative period option: --last-days, --last-weeks, or --this-year")
	}

	nowUTC := now.UTC()
	startToday := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	endToday := startToday.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	if opts.LastDays != 0 {
		if opts.LastDays <= 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("--last-days must be greater than 0")
		}
		from := startToday.AddDate(0, 0, -(opts.LastDays - 1))
		return from, endToday, nil
	}

	if opts.LastWeeks != 0 {
		if opts.LastWeeks <= 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("--last-weeks must be greater than 0")
		}
		days := opts.LastWeeks * 7
		from := startToday.AddDate(0, 0, -(days - 1))
		return from, endToday, nil
	}

	from := time.Date(nowUTC.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	return from, endToday, nil
}
