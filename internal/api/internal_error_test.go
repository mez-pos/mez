package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A client that went away before the answer is not a failure of the server's:
// no 500 is written and nothing is logged at ERROR. A real failure is both.
func TestInternalErrorTellsAGoneClientFromAFailure(t *testing.T) {
	tests := []struct {
		name       string
		cancelled  bool
		wantStatus int
		wantLevel  string
	}{
		{"the client is still there", false, http.StatusInternalServerError, "level=ERROR"},
		{"the client went away", true, http.StatusOK, "level=DEBUG"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			s := &Server{log: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))}
			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancelled {
				cancel()
			} else {
				defer cancel()
			}
			r := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			s.internalError(w, r, "reading the orders failed", errors.New("reading the snapshot: "+context.Canceled.Error()), "venue_id", 7)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tt.wantStatus, w.Body.String())
			}
			if !strings.Contains(buf.String(), tt.wantLevel) {
				t.Fatalf("log = %q, want %s", buf.String(), tt.wantLevel)
			}
			if !strings.Contains(buf.String(), "venue_id=7") || !strings.Contains(buf.String(), "path=/api/v1/orders") {
				t.Fatalf("log = %q, want the extra field and the path", buf.String())
			}
		})
	}
}
