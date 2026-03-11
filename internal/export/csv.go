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

	if err := w.Write([]string{"session_id", "client", "tracking_type", "is_billable", "hourly_rate", "billable_amount", "resource_cost", "monetary_total", "note", "started_at", "stopped_at", "duration_seconds"}); err != nil {
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
			strconv.FormatBool(r.IsBillable),
			strconv.FormatFloat(r.HourlyRate, 'f', 2, 64),
			strconv.FormatFloat(r.BillableAmount, 'f', 2, 64),
			strconv.FormatFloat(r.ResourceCostTotal, 'f', 2, 64),
			strconv.FormatFloat(r.MonetaryTotal, 'f', 2, 64),
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
