package report

import (
	"fmt"
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
