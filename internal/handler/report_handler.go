package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type ReportHandler struct {
	svc service.ReportService
}

func NewReportHandler(svc service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

func (h *ReportHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(constant.UserContextKey).(model.User).ID

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	rangeParam := r.URL.Query().Get("range")
	allTime := r.URL.Query().Get("allTime")

	if allTime != "true" && rangeParam != "all" && (startDate == "" || endDate == "") {
		startDate, endDate = parseRange(rangeParam)
	}

	res, err := h.svc.GetSummary(r.Context(), userID, startDate, endDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.SummaryReport]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

func (h *ReportHandler) GetCategoryBreakdown(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(constant.UserContextKey).(model.User).ID

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	if startDate == "" || endDate == "" {
		rangeParam := r.URL.Query().Get("range")
		startDate, endDate = parseRange(rangeParam)
	}

	res, err := h.svc.GetCategoryBreakdown(r.Context(), userID, startDate, endDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.CategoryBreakdownReport]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

func parseRange(rangeParam string) (string, string) {
	now := time.Now()
	var start, end time.Time

	switch rangeParam {
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = now.AddDate(0, 0, -weekday+1)
		end = start.AddDate(0, 0, 6)
	case "monthly":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "yearly":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(1, 0, 0).Add(-time.Nanosecond)
	default:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	}

	return start.Format("2006-01-02"), end.Format("2006-01-02")
}
