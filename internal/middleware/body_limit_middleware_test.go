package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBodyBytes_RejectsOversizedBody(t *testing.T) {
	h := MaxBodyBytes(64)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]string
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	small := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"a":"b"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, small)
	if rr.Code != http.StatusOK {
		t.Fatalf("small body: expected 200, got %d", rr.Code)
	}

	big := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"`+strings.Repeat("x", 1000)+`"}`))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, big)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("oversized body: expected 400, got %d", rr.Code)
	}
}
