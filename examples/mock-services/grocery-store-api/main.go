package main

import (
	"log"
	"net/http"
	"os"

	"grocery-store-api/handlers"
	"grocery-store-api/middleware"
	"grocery-store-api/store"
)

func main() {
	// Initialize store and seed data
	s := store.NewStore()
	s.Seed()

	// Initialize handlers
	h := handlers.NewHandler(s)

	// Set up router using Go 1.22+ patterns
	mux := http.NewServeMux()

	// Health check (bypasses error injection)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy","service":"grocery-store-api"}`))
	})

	// Admin endpoints for error injection control (bypass middleware)
	mux.HandleFunc("POST /admin/inject-error", middleware.AdminInjectErrorHandler)
	mux.HandleFunc("GET /admin/status", middleware.AdminStatusHandler)
	mux.HandleFunc("POST /admin/reset", middleware.AdminResetHandler)

	// Status
	mux.HandleFunc("GET /status", h.GetStatus)

	// Products
	mux.HandleFunc("GET /products", h.ListProducts)
	mux.HandleFunc("GET /products/{productId}", h.GetProduct)

	// Carts
	mux.HandleFunc("POST /carts", h.CreateCart)
	mux.HandleFunc("GET /carts/{cartId}", h.GetCart)
	mux.HandleFunc("POST /carts/{cartId}/items", h.AddItemToCart)
	mux.HandleFunc("PATCH /carts/{cartId}/items/{itemId}", h.UpdateItemInCart)
	mux.HandleFunc("DELETE /carts/{cartId}/items/{itemId}", h.DeleteItemFromCart)

	// Orders
	mux.HandleFunc("POST /orders", h.CreateOrder)
	mux.HandleFunc("GET /orders/{orderId}", h.GetOrder)

	// Wrap with error injection middleware (excluding health and admin)
	wrappedMux := middleware.ErrorInjectionMiddleware(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("[GROCERY-STORE-API] Starting server on :%s", port)
	log.Println("[GROCERY-STORE-API] Error injection endpoints available:")
	log.Println("  POST /admin/inject-error - Configure error mode")
	log.Println("  GET  /admin/status       - View current config")
	log.Println("  POST /admin/reset        - Reset to normal mode")

	if err := http.ListenAndServe(":"+port, wrappedMux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
