package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type contextKey string

const UserIDContextKey contextKey = "user_id"

type KeyFunc func(r *http.Request) string

func KeyByIP() KeyFunc {
	return func(r *http.Request) string {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(parts[0])
			if clientIP != "" {
				return "ip:" + clientIP
			}
		}

		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return "ip:" + strings.TrimSpace(xri)
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return "ip:" + r.RemoteAddr
		}
		return "ip:" + ip
	}
}

func KeyByHeaderAPIKey(headerName string) KeyFunc {
	if headerName == "" {
		headerName = "X-API-Key"
	}
	return func(r *http.Request) string {
		val := strings.TrimSpace(r.Header.Get(headerName))
		if val == "" {
			return KeyByIP()(r)
		}
		return "apikey:" + val
	}
}

func KeyByUserIDFromContext(ctxKey any) KeyFunc {
	return func(r *http.Request) string {
		if val := r.Context().Value(ctxKey); val != nil {
			if idStr, ok := val.(string); ok && idStr != "" {
				return "user:" + idStr
			}
			return fmt.Sprintf("user:%v", val)
		}
		return KeyByIP()(r)
	}
}

func KeyByRouteAndMethod() KeyFunc {
	return func(r *http.Request) string {
		return fmt.Sprintf("route:%s:%s", r.Method, r.URL.Path)
	}
}

func KeyByRouteAndIP() KeyFunc {
	ipFn := KeyByIP()
	return func(r *http.Request) string {
		return fmt.Sprintf("%s:%s:%s", r.Method, r.URL.Path, ipFn(r))
	}
}

func KeyFuncFromConfig(strategy string, headerName string) KeyFunc {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "header", "api_key", "apikey":
		return KeyByHeaderAPIKey(headerName)

	case "user_id", "user", "user-id":
		return KeyByUserIDFromContext(UserIDContextKey)

	case "route", "endpoint":
		return KeyByRouteAndMethod()

	case "route_ip", "route-ip":
		return KeyByRouteAndIP()

	case "ip":
		fallthrough
	default:
		return KeyByIP()
	}
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDContextKey, userID)
}
