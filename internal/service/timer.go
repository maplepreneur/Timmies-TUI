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

func (s *TimerService) ResumeSession(sessionID int64) (int64, error) {
	return s.store.ResumePausedSession(sessionID, time.Now().UTC())
}

func (s *TimerService) Status() (*sqlite.SessionView, error) {
	return s.store.GetActiveSession()
}

func (s *TimerService) AddSessionResource(sessionID int64, resourceName string, costAmount float64) error {
	return s.store.AddSessionResource(sessionID, resourceName, costAmount)
}

func (s *TimerService) ListSessionResources(sessionID int64) ([]sqlite.SessionResourceView, error) {
	return s.store.ListSessionResources(sessionID)
}

func (s *TimerService) Report(client string, from, to time.Time) ([]sqlite.ReportRow, sqlite.ReportSummary, error) {
	return s.store.ReportByClient(client, from, to)
}

func (s *TimerService) SetBrandingDisplayName(name string) error {
	return s.store.SetBrandingDisplayName(name)
}

func (s *TimerService) SetBrandingLogoPath(path string) error {
	return s.store.SetBrandingLogoPath(path)
}

func (s *TimerService) BrandingSettings() (sqlite.BrandingSettings, error) {
	return s.store.GetBrandingSettings()
}
