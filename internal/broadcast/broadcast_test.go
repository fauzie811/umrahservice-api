package broadcast

import (
	"testing"

	"umrahservice-api/internal/config"
)

func newTestBroadcaster() *Broadcaster {
	return New(&config.Config{
		Reverb: config.ReverbConfig{
			Key:    "testkey",
			Secret: "testsecret",
			AppID:  "1",
			Host:   "localhost",
			Port:   "8080",
			Scheme: "http",
		},
	})
}

func TestAuthResponsePrivate(t *testing.T) {
	b := newTestBroadcaster()
	resp := b.AuthResponse("1234.5678", "private-messages.group_task.5", "")

	// Known vector: HMAC-SHA256("testsecret", "1234.5678:private-messages.group_task.5").
	want := "testkey:ac1018bcb0d5f029bfc6b680079d99062a3bddf022d741e256c18e13559d93c8"
	if resp["auth"] != want {
		t.Fatalf("auth signature = %v, want %v", resp["auth"], want)
	}
	if _, ok := resp["channel_data"]; ok {
		t.Fatalf("private channel must not include channel_data")
	}
}

func TestAuthResponsePresenceIncludesChannelData(t *testing.T) {
	b := newTestBroadcaster()
	resp := b.AuthResponse("1.2", "presence-foo", `{"user_id":1}`)
	if resp["channel_data"] != `{"user_id":1}` {
		t.Fatalf("presence channel must echo channel_data, got %v", resp["channel_data"])
	}
}
