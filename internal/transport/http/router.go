package http

import (
    mw "L0/internal/middleware"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(handler *OrderHandler, _ interface{}, rps float64, burst int, enabled bool) *chi.Mux {
    router := chi.NewRouter()

    router.Use(middleware.Logger)
    router.Use(middleware.RequestID)
    router.Use(mw.NewCustomSlogLogger())
    router.Use(middleware.Recoverer)
    if enabled {
        router.Use(mw.IPRateLimiter(rps, burst))
    }

    // Swagger UI
    router.Get("/swagger/*", httpSwagger.Handler(
        httpSwagger.URL("http://localhost:8081/swagger/doc.json"), // Исправил порт на 8081
    ))

    
    // Json API
    router.Get("/order/{order_uid}", handler.GetOrderByPath)

    // Веб-интерфейс
    router.Get("/", handler.GetOrderPage)

    return router
}