package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"gorm.io/datatypes"
)

func deref(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func itoa(v uint64) string { return strconv.FormatUint(v, 10) }

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseUintPtr(s string) *uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseDate accepts common date / datetime formats sent by clients.
func parseDate(s string) *time.Time {
	layouts := []string{
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return &t
		}
	}
	return nil
}

// formatDateOnly mirrors PHP's $date->format('Y-m-d'); nil-safe.
func formatDateOnly(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func atoiParam(s string) (int, bool) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

func jsonArray(v interface{}) datatypes.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return datatypes.JSON(b)
}
