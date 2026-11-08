package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type stubReportService struct {
	service.ReportService
	summaryFn   func(ctx context.Context, userID uuid.UUID, start, end string) (*dto.SummaryReport, error)
	breakdownFn func(ctx context.Context, userID uuid.UUID, start, end string) (*dto.CategoryBreakdownReport, error)
	trendFn     func(ctx context.Context, userID uuid.UUID, months int) (*dto.MonthlyTrendReport, error)
	netWorthFn  func(ctx context.Context, userID uuid.UUID, months int) (*dto.NetWorthReport, error)
	dailyFn     func(ctx context.Context, userID uuid.UUID, month string) (*dto.DailyReport, error)
}

func (s *stubReportService) GetSummary(ctx context.Context, userID uuid.UUID, start, end string) (*dto.SummaryReport, error) {
	return s.summaryFn(ctx, userID, start, end)
}
func (s *stubReportService) GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, start, end string) (*dto.CategoryBreakdownReport, error) {
	return s.breakdownFn(ctx, userID, start, end)
}
func (s *stubReportService) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.MonthlyTrendReport, error) {
	return s.trendFn(ctx, userID, months)
}
func (s *stubReportService) GetNetWorthTrend(ctx context.Context, userID uuid.UUID, months int) (*dto.NetWorthReport, error) {
	return s.netWorthFn(ctx, userID, months)
}
func (s *stubReportService) GetDailyReport(ctx context.Context, userID uuid.UUID, month string) (*dto.DailyReport, error) {
	return s.dailyFn(ctx, userID, month)
}

func reportParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, util.AppLocation())
	require.NoError(t, err, s)
	return d
}

func TestParseRange(t *testing.T) {
	now := util.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, -1)

	t.Run("weekly is Monday..Sunday containing today", func(t *testing.T) {
		s, e := parseRange("weekly")
		start, end := reportParseDate(t, s), reportParseDate(t, e)
		assert.Equal(t, time.Monday, start.Weekday())
		assert.Equal(t, time.Sunday, end.Weekday())
		assert.Equal(t, start.AddDate(0, 0, 6), end)
		today := reportParseDate(t, util.Now().Format("2006-01-02"))
		assert.False(t, today.Before(start) || today.After(end), "today %s not in [%s,%s]", today, s, e)
	})

	for _, rng := range []string{"monthly", "", "bogus"} {
		t.Run("month for "+fmt.Sprintf("%q", rng), func(t *testing.T) {
			s, e := parseRange(rng)
			assert.Equal(t, monthStart.Format("2006-01-02"), s)
			assert.Equal(t, monthEnd.Format("2006-01-02"), e)
		})
	}

	t.Run("yearly", func(t *testing.T) {
		s, e := parseRange("yearly")
		assert.Equal(t, fmt.Sprintf("%d-01-01", now.Year()), s)
		assert.Equal(t, fmt.Sprintf("%d-12-31", now.Year()), e)
	})
}

func TestReportHandler_GetSummary(t *testing.T) {
	userID := uuid.New()
	ms, me := parseRange("monthly")
	ys, ye := parseRange("yearly")
	tests := []struct {
		name      string
		query     string
		wantStart string
		wantEnd   string
	}{
		{"explicit dates pass through", "?startDate=2026-01-01&endDate=2026-02-01", "2026-01-01", "2026-02-01"},
		{"default range is current month", "", ms, me},
		{"only start falls back to range", "?startDate=2026-01-01&range=yearly", ys, ye},
		{"allTime sends empty dates", "?allTime=true", "", ""},
		{"range=all sends empty dates", "?range=all", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotStart, gotEnd string
			svc := &stubReportService{summaryFn: func(ctx context.Context, uid uuid.UUID, s, e string) (*dto.SummaryReport, error) {
				gotUser, gotStart, gotEnd = uid, s, e
				return &dto.SummaryReport{TotalIncome: 100, TotalExpense: 40}, nil
			}}
			h := NewReportHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.GetSummary(rr, categoryTestReq(http.MethodGet, "/reports/summary"+tt.query, "", userID, ""))

			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, tt.wantStart, gotStart)
			assert.Equal(t, tt.wantEnd, gotEnd)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, float64(100), body["data"].(map[string]interface{})["totalIncome"])
		})
	}
}

func TestReportHandler_GetCategoryBreakdown(t *testing.T) {
	userID := uuid.New()
	ws, we := parseRange("weekly")
	tests := []struct {
		name      string
		query     string
		wantStart string
		wantEnd   string
	}{
		{"explicit dates", "?startDate=2026-03-01&endDate=2026-03-31", "2026-03-01", "2026-03-31"},
		{"range when a date is missing", "?endDate=2026-03-31&range=weekly", ws, we},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotStart, gotEnd string
			svc := &stubReportService{breakdownFn: func(ctx context.Context, uid uuid.UUID, s, e string) (*dto.CategoryBreakdownReport, error) {
				gotUser, gotStart, gotEnd = uid, s, e
				return &dto.CategoryBreakdownReport{TotalExpense: 7}, nil
			}}
			h := NewReportHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.GetCategoryBreakdown(rr, categoryTestReq(http.MethodGet, "/reports/category-breakdown"+tt.query, "", userID, ""))

			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, tt.wantStart, gotStart)
			assert.Equal(t, tt.wantEnd, gotEnd)
			body := decodeJSON(t, rr)
			assert.Equal(t, float64(7), body["data"].(map[string]interface{})["totalExpense"])
		})
	}
}

func TestReportHandler_MonthsParsing(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		query string
		want  int
	}{
		{"", 12},
		{"?months=6", 6},
		{"?months=0", 12},
		{"?months=-3", 12},
		{"?months=abc", 12},
		{"?months=36", 36},
	}
	for _, tt := range tests {
		t.Run("trend"+tt.query, func(t *testing.T) {
			var gotUser uuid.UUID
			got := -1
			svc := &stubReportService{trendFn: func(ctx context.Context, uid uuid.UUID, months int) (*dto.MonthlyTrendReport, error) {
				gotUser, got = uid, months
				return &dto.MonthlyTrendReport{AverageExpense: 3}, nil
			}}
			h := NewReportHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.GetMonthlyTrend(rr, categoryTestReq(http.MethodGet, "/reports/monthly-trend"+tt.query, "", userID, ""))
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, float64(3), decodeJSON(t, rr)["data"].(map[string]interface{})["averageExpense"])
		})
		t.Run("networth"+tt.query, func(t *testing.T) {
			var gotUser uuid.UUID
			got := -1
			svc := &stubReportService{netWorthFn: func(ctx context.Context, uid uuid.UUID, months int) (*dto.NetWorthReport, error) {
				gotUser, got = uid, months
				return &dto.NetWorthReport{Current: 9}, nil
			}}
			h := NewReportHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.GetNetWorthTrend(rr, categoryTestReq(http.MethodGet, "/reports/net-worth"+tt.query, "", userID, ""))
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, float64(9), decodeJSON(t, rr)["data"].(map[string]interface{})["current"])
		})
	}
}

func TestReportHandler_GetDailyReport(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name       string
		query      string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantMonth  string
	}{
		{"success", "?month=2026-11", nil, http.StatusOK, "", "2026-11"},
		{"default month is empty", "", nil, http.StatusOK, "", ""},
		{"invalid month", "?month=11-2026", fmt.Errorf("month: %w", constant.ErrInvalidInput), http.StatusBadRequest, "Invalid month, expected YYYY-MM", "11-2026"},
		{"service error", "?month=2026-11", errors.New("db"), http.StatusInternalServerError, "Failed to get daily report", "2026-11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			gotMonth := "unset"
			svc := &stubReportService{dailyFn: func(ctx context.Context, uid uuid.UUID, month string) (*dto.DailyReport, error) {
				gotUser, gotMonth = uid, month
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &dto.DailyReport{Month: "2026-11"}, nil
			}}
			h := NewReportHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.GetDailyReport(rr, categoryTestReq(http.MethodGet, "/reports/daily"+tt.query, "", userID, ""))

			assert.Equal(t, userID, gotUser)
			assert.Equal(t, tt.wantMonth, gotMonth)
			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "2026-11", decodeJSON(t, rr)["data"].(map[string]interface{})["month"])
		})
	}
}

func TestReportHandler_UnauthorizedAndServiceErrors(t *testing.T) {
	failing := &stubReportService{
		summaryFn: func(ctx context.Context, uid uuid.UUID, s, e string) (*dto.SummaryReport, error) {
			return nil, errors.New("db")
		},
		breakdownFn: func(ctx context.Context, uid uuid.UUID, s, e string) (*dto.CategoryBreakdownReport, error) {
			return nil, errors.New("db")
		},
		trendFn: func(ctx context.Context, uid uuid.UUID, months int) (*dto.MonthlyTrendReport, error) {
			return nil, errors.New("db")
		},
		netWorthFn: func(ctx context.Context, uid uuid.UUID, months int) (*dto.NetWorthReport, error) {
			return nil, errors.New("db")
		},
		dailyFn: func(ctx context.Context, uid uuid.UUID, month string) (*dto.DailyReport, error) {
			return nil, errors.New("db")
		},
	}
	h := NewReportHandler(failing, zap.NewNop())
	endpoints := []struct {
		name    string
		call    http.HandlerFunc
		failMsg string
	}{
		{"summary", h.GetSummary, "Failed to get summary"},
		{"breakdown", h.GetCategoryBreakdown, "Failed to get category breakdown"},
		{"trend", h.GetMonthlyTrend, "Failed to get monthly trend"},
		{"networth", h.GetNetWorthTrend, "Failed to get net worth trend"},
		{"daily", h.GetDailyReport, "Failed to get daily report"},
	}
	for _, ep := range endpoints {
		t.Run(ep.name+" unauthenticated", func(t *testing.T) {
			rr := httptest.NewRecorder()
			ep.call(rr, categoryTestReq(http.MethodGet, "/reports", "", uuid.Nil, ""))
			categoryAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
		})
		t.Run(ep.name+" service error", func(t *testing.T) {
			rr := httptest.NewRecorder()
			ep.call(rr, categoryTestReq(http.MethodGet, "/reports", "", uuid.New(), ""))
			categoryAssertError(t, rr, http.StatusInternalServerError, ep.failMsg)
		})
	}
}
