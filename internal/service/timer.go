package service

import (
	"time"

	"github.com/maplepreneur/chrono/internal/store/sqlite"
)

type TimerService struct {
	store *sqlite.Store
}

func NewTimerService(store *sqlite.Store) *TimerService {
	return &TimerService{store: store}
}

func (s *TimerService) Start(client, trackingType, note string) (int64, error) {
	return s.store.StartSession(client, trackingType, note, time.Now().UTC())
}

func (s *TimerService) Stop() (int64, error) {
	return s.store.StopActiveSession(time.Now().UTC())
}

func (s *TimerService) Resume() (int64, error) {
	return s.store.ResumeLatest(time.Now().UTC())
}

func (s *TimerService) Status() (*sqlite.SessionView, error) {
	return s.store.GetActiveSession()
}

func (s *TimerService) Report(client string, from, to time.Time) ([]sqlite.ReportRow, int64, error) {
	return s.store.ReportByClient(client, from, to)
}
