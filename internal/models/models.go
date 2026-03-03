package models

type Target struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Method   string `json:"method"`
	Interval int    `json:"interval"` // in seconds
	Enabled  bool   `json:"enabled"`
}
