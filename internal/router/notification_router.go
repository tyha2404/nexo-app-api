package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

// NotificationRouter handles routing for notification endpoints
type NotificationRouter struct {
	notificationHandler *handler.NotificationHandler
}

// NewNotificationRouter creates a new instance of NotificationRouter
func NewNotificationRouter(notificationHandler *handler.NotificationHandler) *NotificationRouter {
	return &NotificationRouter{
		notificationHandler: notificationHandler,
	}
}

// RegisterRoutes registers all notification routes
func (r *NotificationRouter) RegisterRoutes(router chi.Router) {
	router.Route("/notifications", func(subRouter chi.Router) {
		// Protected routes
		subRouter.Group(func(protected chi.Router) {
			protected.Use(middleware.AuthMiddleware)

			protected.Get("/vapid-public-key", r.notificationHandler.GetVapidPublicKey)
			protected.Post("/subscribe", r.notificationHandler.Subscribe)
			protected.Post("/unsubscribe", r.notificationHandler.Unsubscribe)
			protected.Post("/test", r.notificationHandler.SendTestNotification)
		})
	})
}
