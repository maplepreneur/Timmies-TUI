package domain

import "time"

type Client struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type TrackingType struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type Session struct {
	ID             int64
	ClientID       int64
	TrackingTypeID int64
	Note           string
	StartedAt      time.Time
	StoppedAt      *time.Time
	DurationSec    int64
	Status         string
	CreatedAt      time.Time
}
