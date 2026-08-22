package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingerStub struct{ err error }

func (p pingerStub) PingContext(context.Context) error { return p.err }

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		pinger     pingerStub
		wantStatus int
		wantBody   string
	}{
		{name: "health is ready", path: "/actuator/health", wantStatus: http.StatusOK, wantBody: `{"status":"UP"}`},
		{name: "readiness reports database failure", path: "/actuator/health/readiness", pinger: pingerStub{err: errors.New("unavailable")}, wantStatus: http.StatusServiceUnavailable, wantBody: `{"status":"DOWN"}`},
		{name: "liveness does not depend on database", path: "/actuator/health/liveness", pinger: pingerStub{err: errors.New("unavailable")}, wantStatus: http.StatusOK, wantBody: `{"status":"UP"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter("test", tt.pinger)
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if body := response.Body.String(); body != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
