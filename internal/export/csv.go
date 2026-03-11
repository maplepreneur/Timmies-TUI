package export

import (
	"encoding/csv"
	"os"
	"strconv"

	"github.com/maplepreneur/chrono/internal/store/sqlite"
)

func WriteReportCSV(path string, rows []sqlite.ReportRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"session_id", "client", "tracking_type", "note", "started_at", "stopped_at", "duration_seconds"}); err != nil {
		return err
	}
	for _, r := range rows {
		stopped := ""
		if r.StoppedAt != nil {
			stopped = r.StoppedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if err := w.Write([]string{
			strconv.FormatInt(r.SessionID, 10),
			r.ClientName,
			r.TrackingTypeName,
			r.Note,
			r.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			stopped,
			strconv.FormatInt(r.ComputedDurationS, 10),
		}); err != nil {
			return err
		}
	}
	return w.Error()
}
