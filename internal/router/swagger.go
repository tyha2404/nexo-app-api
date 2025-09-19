package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

// AddSwaggerRoute adds the Swagger UI route to the router
func AddSwaggerRoute(r *chi.Mux) {
	// Serve Swagger UI
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Serve the generated Swagger docs (doc.json, etc.)
	r.Get("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/swagger.json")
	})
}
