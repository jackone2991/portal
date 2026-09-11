package handler

import (
	"crypto/subtle"
	"github.com/portal/backend/internal/platform/server"
	"net"
	"net/http"
)

// writeError answers with RFC 7807. The legacy {code, message} body this used to
// write is retired (ADR-10); `code` is carried through as the problem type so
// every existing call site keeps its vocabulary and gains the standard shape.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	server.Problem(w, status, server.ProblemType("account", code), http.StatusText(status), msg)
}

// subtleEqual is a constant-time string compare, returning true on match.
// Use for any value an attacker can repeatedly probe (state, tokens, hmacs).
func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func nonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// clientIP follows the same algorithm as the rate-limit middleware.
// Duplicated here to avoid an import cycle (handler ↔ middleware).
func clientIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		s := xff
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				s = xff[:i]
				break
			}
		}
		if ip := net.ParseIP(trim(s)); ip != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
