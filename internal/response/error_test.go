package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%q)", err, rec.Body.String())
	}
	return body
}

func TestSendError_ClientErrorIncludesDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	SendError(rec, http.StatusBadRequest, "Invalid request body", errors.New("unexpected EOF"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	body := decodeBody(t, rec)
	if body["status"] != float64(400) || body["success"] != false {
		t.Fatalf("unexpected envelope: %v", body)
	}
	if body["message"] != "Invalid request body" {
		t.Fatalf("message = %v", body["message"])
	}
	if body["error"] != "unexpected EOF" {
		t.Fatalf("error = %v, want detail for 4xx", body["error"])
	}
}

func TestSendError_ServerErrorHidesDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	SendError(rec, http.StatusInternalServerError, "Failed to list transactions", errors.New("pq: relation \"transactions\" does not exist"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["message"] != "Failed to list transactions" {
		t.Fatalf("message = %v", body["message"])
	}
	if _, ok := body["error"]; ok {
		t.Fatalf("error field must be omitted for 5xx, got %v", body["error"])
	}
}

func TestSendError_NilError(t *testing.T) {
	rec := httptest.NewRecorder()
	SendError(rec, http.StatusNotFound, "Not found", nil)

	body := decodeBody(t, rec)
	if _, ok := body["error"]; ok {
		t.Fatalf("error field must be omitted when err is nil")
	}
	if body["message"] != "Not found" || body["status"] != float64(404) {
		t.Fatalf("unexpected envelope: %v", body)
	}
}
