package model

import "time"

type VerificationEvent struct {
	EventID      string    `json:"event_id"`
	CreatedAt    time.Time `json:"created_at"`
	Source       string    `json:"source_service"`
	Endpoint     string    `json:"endpoint"`
	RequestID    string    `json:"request_id,omitempty"`
	VUZID        string    `json:"vuz_id,omitempty"`
	VUZCode      string    `json:"vuz_code,omitempty"`
	DiplomaHash  string    `json:"diploma_hash,omitempty"`
	Status       string    `json:"status"`
	Valid        bool      `json:"is_valid"`
	Country      string    `json:"country,omitempty"`
	City         string    `json:"city,omitempty"`
	ClientIPHash string    `json:"client_ip_hash,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
}

type VerificationStatsFilter struct {
	From time.Time
	To   time.Time
}

type VerificationStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type VerificationTimeBucket struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type VerificationGeoPoint struct {
	Country string `json:"country,omitempty"`
	City    string `json:"city,omitempty"`
	Count   int64  `json:"count"`
}

type VerificationTopUniversity struct {
	VUZID   string `json:"vuz_id,omitempty"`
	VUZCode string `json:"vuz_code,omitempty"`
	Name    string `json:"name,omitempty"`
	Checks  int64  `json:"checks"`
}

type VerificationStatsResponse struct {
	From             time.Time                   `json:"from"`
	To               time.Time                   `json:"to"`
	TotalChecks      int64                       `json:"total_checks"`
	UniqueRequesters int64                       `json:"unique_requesters"`
	Statuses         []VerificationStatusCount   `json:"statuses"`
	Timeseries       []VerificationTimeBucket    `json:"timeseries"`
	Geography        []VerificationGeoPoint      `json:"geography"`
	TopUniversities  []VerificationTopUniversity `json:"top_universities,omitempty"`
}
