package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type TargetRouter struct {
	handler *handler.TargetHandler
}

func NewTargetRouter(handler *handler.TargetHandler) *TargetRouter {
	return &TargetRouter{handler: handler}
}

func (r *TargetRouter) RegisterRoutes(router chi.Router) {
	router.Route("/targets", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Post("/monthly", r.handler.UpsertTarget)
		router.Get("/summary", r.handler.GetSummary)
	})
}
