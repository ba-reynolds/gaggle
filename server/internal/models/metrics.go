package models

import (
	"time"

	"github.com/ba-reynolds/gaggle/internal/metrics"
)

// AdminMetrics is the full snapshot served by GET /admin/metrics.
type AdminMetrics struct {
	Host   metrics.HostStats `json:"host"`
	App    AppStats          `json:"app"`
	Active ActiveUsers       `json:"active"`
	Views  ViewStats         `json:"views"`
}

// AppStats aggregates platform-wide counters.
type AppStats struct {
	Users      int `json:"users"`
	Posts      int `json:"posts"`
	Likes      int `json:"likes"`
	Messages   int `json:"messages"`
	ViewsTotal int `json:"views_total"`
	Signups24h int `json:"signups_24h"`
}

// ActiveUsers counts distinct logged-in visitors over time windows.
type ActiveUsers struct {
	DAU int `json:"dau"`
	WAU int `json:"wau"`
}

// ViewStats is traffic derived from the page_views table.
type ViewStats struct {
	// RequestsPerMinute is the number of recorded views in the last 60s.
	RequestsPerMinute float64        `json:"requests_per_minute"`
	ByDay             []DayViewCount `json:"by_day"`
}

// DayViewCount is one day of view traffic (day is YYYY-MM-DD, UTC).
type DayViewCount struct {
	Day   string `json:"day"`
	Views int    `json:"views"`
}

// HistoryRange selects how far back /admin/metrics/history looks.
type HistoryRange string

const (
	History24h HistoryRange = "24h"
	History7d  HistoryRange = "7d"
	History30d HistoryRange = "30d"
)

// Valid reports whether the range is one the API supports.
func (r HistoryRange) Valid() bool {
	switch r {
	case History24h, History7d, History30d:
		return true
	}
	return false
}

// HostSamplePoint is one (possibly downsampled) row of host history.
type HostSamplePoint struct {
	Timestamp   time.Time `json:"ts"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	DiskPercent float64   `json:"disk_percent"`
	Load1       float64   `json:"load1"`
}

// MetricsHistory is the payload of GET /admin/metrics/history.
type MetricsHistory struct {
	Range HistoryRange      `json:"range"`
	Days  int               `json:"days"`
	Host  []HostSamplePoint `json:"host"`
	Views []DayViewCount    `json:"views"`
}
