package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/patrickkabwe/grx/core"
)

// randRead injects entropy for request IDs (overridden in tests).
var randRead = rand.Read

// RequestID returns middleware that ensures each request carries a request ID
// in context ([core.RequestIDFromContext]) and echoes it on the response.
// When header is empty, "X-Request-Id" is used. If the incoming request omits
// the header, a random ID is generated.
func RequestID(header string) Middleware {
	if strings.TrimSpace(header) == "" {
		header = "X-Request-Id"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimSpace(r.Header.Get(header))
			if id == "" {
				id = randomRequestID()
			}
			ctx := core.WithRequestID(r.Context(), id)
			r = r.WithContext(ctx)
			w.Header().Set(header, id)
			next.ServeHTTP(w, r)
		})
	}
}

func randomRequestID() string {
	var buf [8]byte
	if _, err := randRead(buf[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf[:])
}
