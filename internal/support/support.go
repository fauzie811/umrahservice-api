// Package support holds small shared helpers (period lookup, token gen,
// date formatting) used across handlers.
package support

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const tokenAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// CurrentPeriodID mirrors current_period(): reads GeneralSettings.period_id
// from the `settings` table (group=general, name=period_id), defaulting to 1.
func CurrentPeriodID(db *gorm.DB) uint64 {
	var payload string
	err := db.Table("settings").
		Select("payload").
		Where("`group` = ? AND name = ?", "general", "period_id").
		Scan(&payload).Error
	if err == nil && payload != "" {
		var id uint64
		if json.Unmarshal([]byte(payload), &id) == nil && id != 0 {
			return id
		}
	}
	return 1
}

// RandomToken returns a 40-char alphanumeric string (Str::random(40)).
func RandomToken() string {
	b := make([]byte, 40)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = tokenAlphabet[int(b[i])%len(tokenAlphabet)]
	}
	return string(b)
}

// HashToken returns the sha256 hex of a plaintext token (Sanctum scheme).
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// ISO8601 formats a time like Carbon::toIso8601String ("2006-01-02T15:04:05-07:00").
func ISO8601(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02T15:04:05-07:00")
	return &s
}

// ISO formats a time like Carbon's default JSON (RFC3339 with microseconds, Z).
func ISO(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02T15:04:05.000000Z")
	return &s
}
