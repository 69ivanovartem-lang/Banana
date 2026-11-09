package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func corsMiddleware(next http.Handler) http.Handler {
	origin := os.Getenv("BANANA_FRONTEND_ORIGIN")
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type respCap struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (rc *respCap) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *respCap) Write(b []byte) (int, error) {
	if rc.buf.Len() < 2048 {
		rc.buf.Write(b)
	}
	return rc.ResponseWriter.Write(b)
}

func fullLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var reqPreview string
		if r.Body != nil && !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			body, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
			reqPreview = string(body)
			r.Body = io.NopCloser(bytes.NewReader(body))
		} else if r.Body != nil {
			reqPreview = "[multipart/form-data]"
		}
		rc := &respCap{ResponseWriter: w, status: http.StatusOK, buf: &bytes.Buffer{}}
		next.ServeHTTP(rc, r)
		respPreview := rc.buf.String()
		if len(respPreview) > 512 {
			respPreview = respPreview[:512] + "..."
		}
		if len(reqPreview) > 512 {
			reqPreview = reqPreview[:512] + "..."
		}
		slog.Info("HTTP",
			"method", r.Method, "path", r.URL.Path, "query", r.URL.RawQuery,
			"status", rc.status, "duration", time.Since(start).String(),
			"req_body", reqPreview, "resp_body", respPreview,
		)
	})
}

type contextKey string

const userIDContextKey contextKey = "userID"

func requireAuth(store *Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "authorization required")
			return
		}
		userID, ok := store.UserIDByToken(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next(w, r.WithContext(ctx))
	}
}

func userIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDContextKey).(string); ok {
		return v
	}
	return ""
}
