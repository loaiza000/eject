package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSyntheticHandler(t *testing.T) {
	t.Setenv("DATABASE_HOST", "")
	for _, tc := range []struct {
		path string
		code int
		body string
	}{{"/", http.StatusOK, "EJECT synthetic HTTP demo"}, {"/health", http.StatusServiceUnavailable, "database host missing"}} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		resp := httptest.NewRecorder()
		newHandler().ServeHTTP(resp, req)
		if resp.Code != tc.code || !strings.Contains(resp.Body.String(), tc.body) {
			t.Fatalf("%s: code=%d body=%q", tc.path, resp.Code, resp.Body.String())
		}
	}
}
