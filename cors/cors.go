// Package cors provides HTTP middleware for CORS and WebSocket Origin checks.
package cors

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Config configures CORS middleware.
type Config struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// New returns middleware that handles HTTP CORS and WebSocket Origin checks.
func New(config Config) func(http.Handler) http.Handler {
	policy := newPolicy(config)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if strings.TrimSpace(origin) == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !policy.allowsOrigin(origin) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}

			policy.writeSimpleHeaders(w, origin)
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if !policy.allowsMethod(r.Header.Get("Access-Control-Request-Method")) {
					http.Error(w, "method not allowed", http.StatusForbidden)
					return
				}
				if !policy.allowsHeaders(r.Header.Values("Access-Control-Request-Headers")) {
					http.Error(w, "headers not allowed", http.StatusForbidden)
					return
				}
				policy.writePreflightHeaders(w)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type policy struct {
	allowedOrigins   map[string]struct{}
	allowedMethods   map[string]struct{}
	allowedHeaders   map[string]struct{}
	exposedHeaders   string
	allowAnyOrigin   bool
	allowAnyHeader   bool
	allowCredentials bool
	maxAge           time.Duration
	methodHeader     string
	headerHeader     string
}

func newPolicy(config Config) policy {
	allowAnyOrigin := containsString(config.AllowedOrigins, "*", false)
	if config.AllowCredentials {
		allowAnyOrigin = false
	}
	return policy{
		allowedOrigins:   stringSet(config.AllowedOrigins, false),
		allowedMethods:   methodSet(config.AllowedMethods),
		allowedHeaders:   stringSet(config.AllowedHeaders, true),
		exposedHeaders:   strings.Join(config.ExposedHeaders, ", "),
		allowAnyOrigin:   allowAnyOrigin,
		allowAnyHeader:   containsString(config.AllowedHeaders, "*", true),
		allowCredentials: config.AllowCredentials,
		maxAge:           config.MaxAge,
		methodHeader:     strings.Join(config.AllowedMethods, ", "),
		headerHeader:     strings.Join(config.AllowedHeaders, ", "),
	}
}

func methodSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := strings.ToUpper(strings.TrimSpace(value))
		if key != "" {
			set[key] = struct{}{}
		}
	}
	return set
}

func stringSet(values []string, canonicalHeader bool) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if canonicalHeader {
			key = http.CanonicalHeaderKey(key)
		}
		if key != "" {
			set[key] = struct{}{}
		}
	}
	return set
}

func containsString(values []string, needle string, canonicalHeader bool) bool {
	for _, value := range values {
		key := strings.TrimSpace(value)
		if canonicalHeader {
			key = http.CanonicalHeaderKey(key)
		}
		if key == needle {
			return true
		}
	}
	return false
}

func (p policy) allowsOrigin(origin string) bool {
	if p.allowAnyOrigin {
		return true
	}
	_, ok := p.allowedOrigins[origin]
	return ok
}

func (p policy) allowsMethod(method string) bool {
	_, ok := p.allowedMethods[strings.ToUpper(strings.TrimSpace(method))]
	return ok
}

func (p policy) allowsHeaders(values []string) bool {
	if p.allowAnyHeader {
		return true
	}
	for _, value := range values {
		for _, header := range strings.Split(value, ",") {
			key := http.CanonicalHeaderKey(strings.TrimSpace(header))
			if key == "" {
				continue
			}
			if _, ok := p.allowedHeaders[key]; !ok {
				return false
			}
		}
	}
	return true
}

func (p policy) writeSimpleHeaders(w http.ResponseWriter, origin string) {
	if p.allowAnyOrigin && !p.allowCredentials {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	if p.allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if p.exposedHeaders != "" {
		w.Header().Set("Access-Control-Expose-Headers", p.exposedHeaders)
	}
}

func (p policy) writePreflightHeaders(w http.ResponseWriter) {
	if p.methodHeader != "" {
		w.Header().Set("Access-Control-Allow-Methods", p.methodHeader)
	}
	if p.headerHeader != "" {
		w.Header().Set("Access-Control-Allow-Headers", p.headerHeader)
	}
	if p.maxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.FormatInt(int64(p.maxAge.Seconds()), 10))
	}
}
