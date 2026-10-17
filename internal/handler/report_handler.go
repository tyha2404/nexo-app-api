package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type ReportHandler struct {
	svc          service.ReportService
	errorHandler *ErrorHandler
}

func NewReportHandler(svc service.ReportService, log *zap.Logger) *ReportHandler {
	return &ReportHandler{svc: svc, errorHandler: NewErrorHandler(log)}
}

func (h *ReportHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_summary")
		return
	}
	userID := user.ID

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	rangeParam := r.URL.Query().Get("range")
	allTime := r.URL.Query().Get("allTime")

	if allTime != "true" && rangeParam != "all" && (startDate == "" || endDate == "") {
		startDate, endDate = parseRange(rangeParam)
	}

	res, err := h.svc.GetSummary(r.Context(), userID, startDate, endDate)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get summary", "get_summary")
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
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_category_breakdown")
		return
	}
	userID := user.ID

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	if startDate == "" || endDate == "" {
		rangeParam := r.URL.Query().Get("range")
		startDate, endDate = parseRange(rangeParam)
	}

	res, err := h.svc.GetCategoryBreakdown(r.Context(), userID, startDate, endDate)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get category breakdown", "get_category_breakdown")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.CategoryBreakdownReport]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

// GetMonthlyTrend handles GET /reports/monthly-trend
// @Summary Get monthly financial trend
// @Description Get 12-month expense, income, target and average trend
// @Tags Reports
// @Accept json
// @Produce json
// @Param months query int false "Number of months (default: 12)"
// @Security BearerAuth
// @Success 200 {object} MonthlyTrendResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /reports/monthly-trend [get]
func (h *ReportHandler) GetMonthlyTrend(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_monthly_trend")
		return
	}
	userID := user.ID

	monthsStr := r.URL.Query().Get("months")
	months := 12
	if monthsStr != "" {
		if parsed, err := strconv.Atoi(monthsStr); err == nil && parsed > 0 {
			months = parsed
		}
	}

	res, err := h.svc.GetMonthlyTrend(r.Context(), userID, months)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get monthly trend", "get_monthly_trend")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.MonthlyTrendReport]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}

func parseRange(rangeParam string) (string, string) {
	now := util.Now()
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

// MonthlyTrendResponse describes the report envelope for Swagger.
type MonthlyTrendResponse struct {
	Status  int                    `json:"status"`
	Success bool                   `json:"success"`
	Data    dto.MonthlyTrendReport `json:"data"`
}
