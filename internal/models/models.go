package models

import "time"

type Target struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Method   string `json:"method"`
	Interval int    `json:"interval"` // in seconds
	Enabled  bool   `json:"enabled"`
}

type Check struct {
	ID        string    `json:"id"`
	TargetID  string    `json:"target_id"`
	OK        bool      `json:"ok"`
	LatencyMS int       `json:"latency_ms"` // in milliseconds
	ErrorMsg  string    `json:"error_msg,omitempty"`
	Timestamp time.Time `json:"timestamp"` // Unix timestamp
}
