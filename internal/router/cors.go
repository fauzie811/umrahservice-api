package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// cors handles browser CORS for web clients. Because the API uses credentials
// (the XSRF-TOKEN cookie + Authorization header), the spec forbids the wildcard
// origin: a matching request Origin is reflected back and credentials enabled.
//
// allowed lists the permitted origins. When empty, any origin is reflected
// (convenient for local dev) — still with credentials, which is acceptable here
// because the API authenticates every protected route via Bearer token.
func cors(allowed []string) gin.HandlerFunc {
	allowSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowSet[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (len(allowSet) == 0 || allowSet[origin]) {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With, X-XSRF-TOKEN")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
