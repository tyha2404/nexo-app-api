package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type PushRouter struct {
	handler *handler.PushHandler
}

func NewPushRouter(handler *handler.PushHandler) *PushRouter {
	return &PushRouter{handler: handler}
}

func (r *PushRouter) RegisterRoutes(router chi.Router) {
	router.Route("/push", func(router chi.Router) {
		// Public: the key is not secret and is needed before subscribing.
		router.Get("/vapid-public-key", r.handler.GetVAPIDPublicKey)
		router.Group(func(router chi.Router) {
			router.Use(middleware.AuthMiddleware)
			router.Post("/subscriptions", r.handler.Subscribe)
			router.Delete("/subscriptions", r.handler.Unsubscribe)
		})
	})
}
