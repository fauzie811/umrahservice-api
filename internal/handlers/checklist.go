package handlers

import (
	"bytes"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

func nopCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
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
