package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// bindJSONorForm fills a generic map from either a JSON body or form data,
// preserving key presence so Laravel's `sometimes` semantics can be replicated.
func bindJSONorForm(c *gin.Context, out *map[string]interface{}) error {
	m := map[string]interface{}{}
	if strings.Contains(c.ContentType(), "application/json") {
		if b := readBody(c); len(b) > 0 {
			_ = json.Unmarshal(b, &m)
		}
	} else {
		_ = c.Request.ParseMultipartForm(32 << 20)
		if c.Request.PostForm != nil {
			for k, v := range c.Request.PostForm {
				if len(v) > 0 {
					m[k] = v[0]
				}
			}
		}
	}
	*out = m
	return nil
}

// stringField returns the string value of a map key and whether it was present.
func stringField(m map[string]interface{}, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case nil:
		return "", true
	default:
		return fmt.Sprintf("%v", t), true
	}
}
