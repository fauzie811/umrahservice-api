package handlers

import (
	"encoding/json"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

const jsonBodyCtxKey = "_jsonBody"

// requestField returns a field value from either a JSON request body or
// form/multipart data, so handlers accept both content types uniformly.
func requestField(c *gin.Context, key string) string {
	if m := jsonBody(c); m != nil {
		if v, ok := m[key]; ok {
			return stringifyJSON(v)
		}
		return ""
	}
	return c.PostForm(key)
}

// jsonBody parses and caches the request body as a map when the request is
// application/json. Returns nil for any other content type (e.g. multipart),
// leaving the body untouched for c.FormFile / c.PostForm.
func jsonBody(c *gin.Context) map[string]any {
	if v, ok := c.Get(jsonBodyCtxKey); ok {
		m, _ := v.(map[string]any)
		return m
	}
	if c.ContentType() != "application/json" {
		c.Set(jsonBodyCtxKey, nil)
		return nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Set(jsonBodyCtxKey, nil)
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		c.Set(jsonBodyCtxKey, nil)
		return nil
	}
	c.Set(jsonBodyCtxKey, m)
	return m
}

// stringifyJSON renders a decoded JSON value as the string the form-based
// handlers expect. JSON numbers decode to float64; integral values render
// without a trailing ".0".
func stringifyJSON(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return "0"
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

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
