package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
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

// GetSummary handles GET /reports/summary
// @Summary Get the income/expense summary
// @Description Totals for [startDate, endDate]. Without both dates the period comes from range (weekly, monthly — the default — or yearly) in the app timezone; range=all or allTime=true sums all time.
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param startDate query string false "YYYY-MM-DD"
// @Param endDate query string false "YYYY-MM-DD"
// @Param range query string false "weekly, monthly (default), yearly or all"
// @Param allTime query bool false "true to sum all time"
// @Success 200 {object} SummaryReportEnvelope
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to get summary"
// @Router /reports/summary [get]
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

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// GetCategoryBreakdown handles GET /reports/category-breakdown
// @Summary Get spending by category
// @Description Expense per category for [startDate, endDate]; without both dates the period comes from range (weekly, monthly — the default — or yearly) in the app timezone.
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Param startDate query string false "YYYY-MM-DD"
// @Param endDate query string false "YYYY-MM-DD"
// @Param range query string false "weekly, monthly (default) or yearly"
// @Success 200 {object} CategoryBreakdownEnvelope
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to get category breakdown"
// @Router /reports/category-breakdown [get]
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

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// GetMonthlyTrend handles GET /reports/monthly-trend
// @Summary Get monthly financial trend
// @Description Monthly expense, income, expense target and average expense for the last `months` calendar months (default 12, max 36; values outside 1–36 use 12), ending with the current month in the app timezone.
// @Tags Reports
// @Produce json
// @Param months query int false "Number of months (default: 12, max 36)"
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

	// The service defaults a missing/invalid months to 12.
	res, err := h.svc.GetMonthlyTrend(r.Context(), userID, QueryInt(r, "months"))
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get monthly trend", "get_monthly_trend")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// GetNetWorthTrend handles GET /reports/net-worth
// @Summary Get month-end net worth over time
// @Description Net worth at the end of each of the last `months` months (default 12, max 36), ending with the current month in the app timezone. Net worth = cumulative income − expense + realized investment PnL, the dashboard's "Tổng tài sản". Leading months before the first transaction are omitted (at least two points are kept).
// @Tags Reports
// @Produce json
// @Param months query int false "Number of months (default: 12, max 36)"
// @Security BearerAuth
// @Success 200 {object} NetWorthResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /reports/net-worth [get]
func (h *ReportHandler) GetNetWorthTrend(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_net_worth")
		return
	}

	res, err := h.svc.GetNetWorthTrend(r.Context(), user.ID, QueryInt(r, "months"))
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get net worth trend", "get_net_worth")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// GetDailyReport handles GET /reports/daily
// @Summary Get daily income/expense for a month
// @Description Per-day income, expense and transaction count for every day of the month (days without transactions are zero), for the spending calendar.
// @Tags Reports
// @Produce json
// @Param month query string false "Month as YYYY-MM (default: current month in the app timezone)"
// @Security BearerAuth
// @Success 200 {object} DailyReportResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /reports/daily [get]
func (h *ReportHandler) GetDailyReport(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_daily_report")
		return
	}

	res, err := h.svc.GetDailyReport(r.Context(), user.ID, r.URL.Query().Get("month"))
	if err != nil {
		if errors.Is(err, constant.ErrInvalidInput) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid month, expected YYYY-MM", "get_daily_report")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get daily report", "get_daily_report")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
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

	return start.Format(time.DateOnly), end.Format(time.DateOnly)
}

// MonthlyTrendResponse describes the report envelope for Swagger.
type MonthlyTrendResponse struct {
	Status  int                    `json:"status"`
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    dto.MonthlyTrendReport `json:"data"`
}

// NetWorthResponse describes the net worth report envelope for Swagger.
type NetWorthResponse struct {
	Status  int                `json:"status"`
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    dto.NetWorthReport `json:"data"`
}

// DailyReportResponse describes the daily report envelope for Swagger.
type DailyReportResponse struct {
	Status  int             `json:"status"`
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    dto.DailyReport `json:"data"`
}
