// Package broadcast implements the Pusher/Reverb auth-signature and event-trigger
// protocol used by the Laravel app's broadcasting layer.
package broadcast

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"umrahservice-api/internal/config"
)

// Broadcaster signs channel-auth requests and triggers events on Reverb.
type Broadcaster struct {
	key     string
	secret  string
	appID   string
	baseURL string
	http    *http.Client
}

func New(cfg *config.Config) *Broadcaster {
	base := fmt.Sprintf("%s://%s:%s", cfg.Reverb.Scheme, cfg.Reverb.Host, cfg.Reverb.Port)
	return &Broadcaster{
		key:     cfg.Reverb.Key,
		secret:  cfg.Reverb.Secret,
		appID:   cfg.Reverb.AppID,
		baseURL: base,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// AuthResponse signs a channel subscription. channelData should be empty for
// private channels and a JSON string for presence channels.
func (b *Broadcaster) AuthResponse(socketID, channelName, channelData string) map[string]interface{} {
	payload := socketID + ":" + channelName
	if channelData != "" {
		payload += ":" + channelData
	}
	sig := b.hmacHex(payload)

	resp := map[string]interface{}{"auth": b.key + ":" + sig}
	if channelData != "" {
		resp["channel_data"] = channelData
	}
	return resp
}

// Trigger publishes an event to one channel (best-effort).
func (b *Broadcaster) Trigger(ctx context.Context, channel, event string, data interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]interface{}{
		"name":     event,
		"channels": []string{channel},
		"data":     string(dataJSON),
	})
	if err != nil {
		return err
	}

	path := "/apps/" + b.appID + "/events"
	bodyMD5 := fmt.Sprintf("%x", md5.Sum(body))
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	params := map[string]string{
		"auth_key":       b.key,
		"auth_timestamp": timestamp,
		"auth_version":   "1.0",
		"body_md5":       bodyMD5,
	}
	query := sortedQuery(params)
	stringToSign := "POST\n" + path + "\n" + query
	signature := b.hmacHex(stringToSign)

	url := b.baseURL + path + "?" + query + "&auth_signature=" + signature
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (b *Broadcaster) hmacHex(payload string) string {
	mac := hmac.New(sha256.New, []byte(b.secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func sortedQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}
