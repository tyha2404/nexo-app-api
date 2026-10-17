package response

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Status  int    `json:"status"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// SendError writes the standard JSON error envelope.
//
// The underlying error text is only exposed for client errors (4xx), where it
// carries useful detail such as validation failures. For server errors (5xx)
// the `error` field is omitted so internal details (SQL, driver, upstream
// messages) never leak to clients; callers are expected to log err instead.
func SendError(w http.ResponseWriter, status int, message string, err error) {
	response := ErrorResponse{
		Status:  status,
		Success: false,
		Message: message,
	}

	if err != nil && status < http.StatusInternalServerError {
		response.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
