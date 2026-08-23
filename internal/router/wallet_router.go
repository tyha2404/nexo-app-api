package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
)

type WalletRouter struct {
	handler *handler.WalletHandler
}

func NewWalletRouter(handler *handler.WalletHandler) *WalletRouter {
	return &WalletRouter{handler: handler}
}

func (r *WalletRouter) RegisterRoutes(router chi.Router) {
	router.Route("/wallets", func(router chi.Router) {
		router.Use(middleware.AuthMiddleware)
		router.Get("/", r.handler.GetWallets)
		router.Post("/", r.handler.CreateWallet)
		router.Post("/transfer", r.handler.TransferMoney)
		router.Post("/auto-allocate", r.handler.AutoAllocateIncome)
		router.Get("/{id}", r.handler.GetWalletByID)
		router.Put("/{id}", r.handler.UpdateWallet)
		router.Delete("/{id}", r.handler.DeleteWallet)
	})
}
