package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

func nopCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

func unmarshalChecklist(b []byte, out *[]map[string]interface{}) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return json.Unmarshal(b, out)
}

// truthy mirrors PHP's (bool) cast for checklist done flags.
func truthy(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case int:
		return t != 0
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	}
	return false
}

// parseChecklistBools extracts the request's checklist booleans, keyed by index.
// Supports a JSON body ({"checklist":[true,false]} or {"checklist":{"0":true}})
// and multipart/form-urlencoded keys (checklist[0]=1).
func parseChecklistBools(c *gin.Context) (map[int]bool, bool) {
	out := map[int]bool{}
	present := false

	ct := c.ContentType()
	if strings.Contains(ct, "application/json") {
		var body struct {
			Checklist json.RawMessage `json:"checklist"`
		}
		if err := json.Unmarshal(readBody(c), &body); err == nil && len(body.Checklist) > 0 {
			present = true
			// Try array form.
			var arr []interface{}
			if json.Unmarshal(body.Checklist, &arr) == nil {
				for i, v := range arr {
					out[i] = truthy(v)
				}
				return out, present
			}
			// Try map form.
			var m map[string]interface{}
			if json.Unmarshal(body.Checklist, &m) == nil {
				for k, v := range m {
					if i, err := strconv.Atoi(k); err == nil {
						out[i] = truthy(v)
					}
				}
			}
		}
		return out, present
	}

	// Form-encoded: checklist[<i>]=<bool>
	_ = c.Request.ParseMultipartForm(32 << 20)
	if c.Request.PostForm != nil {
		for key, vals := range c.Request.PostForm {
			if strings.HasPrefix(key, "checklist[") && strings.HasSuffix(key, "]") {
				idxStr := key[len("checklist[") : len(key)-1]
				if i, err := strconv.Atoi(idxStr); err == nil && len(vals) > 0 {
					present = true
					out[i] = truthy(vals[0])
				}
			}
		}
	}
	return out, present
}

// readBody reads and restores the request body so it can be read again.
func readBody(c *gin.Context) []byte {
	if c.Request.Body == nil {
		return nil
	}
	b, err := c.GetRawData()
	if err != nil {
		return nil
	}
	// Restore for any later reads.
	c.Request.Body = nopCloser(b)
	return b
}

// rawJSONOrEmpty decodes JSON or returns an empty array (mirrors `?? []`).
func rawJSONOrEmpty(j datatypes.JSON) interface{} {
	if len(j) == 0 {
		return []interface{}{}
	}
	v := rawJSON(j)
	if v == nil {
		return []interface{}{}
	}
	return v
}
