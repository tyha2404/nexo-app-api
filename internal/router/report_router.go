package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type ReportRouter struct {
	handler *handler.ReportHandler
}

func NewReportRouter(handler *handler.ReportHandler) *ReportRouter {
	return &ReportRouter{handler: handler}
}

func (r *ReportRouter) RegisterRoutes(router chi.Router) {
	router.Route("/reports", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Get("/summary", r.handler.GetSummary)
		router.Get("/category-breakdown", r.handler.GetCategoryBreakdown)
	})
}
