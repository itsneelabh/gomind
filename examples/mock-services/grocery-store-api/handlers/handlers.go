package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"grocery-store-api/store"
)

type Handler struct {
	Store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "UP"})
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	resultsStr := r.URL.Query().Get("results")
	limit := 0
	if resultsStr != "" {
		if l, err := strconv.Atoi(resultsStr); err == nil {
			limit = l
		}
	}

	products := h.Store.GetProducts(category, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("productId")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	product, ok := h.Store.GetProduct(id)
	if !ok {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

func (h *Handler) CreateCart(w http.ResponseWriter, r *http.Request) {
	cartID := h.Store.CreateCart()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"created": true,
		"cartId":  cartID,
	})
}

func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")
	cart, ok := h.Store.GetCart(cartID)
	if !ok {
		http.Error(w, "Cart not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
}

func (h *Handler) AddItemToCart(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")
	var req struct {
		ProductID int `json:"productId"`
		Quantity  int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	itemID, err := h.Store.AddItemToCart(cartID, req.ProductID, req.Quantity)
	if err != nil {
		if err.Error() == "cart not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"created": true,
		"itemId":  itemID,
	})
}

func (h *Handler) UpdateItemInCart(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")
	itemIDStr := r.PathValue("itemId")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.Store.UpdateItemInCart(cartID, itemID, req.Quantity); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound) // Assuming mostly not found errors for simplicity
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteItemFromCart(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")
	itemIDStr := r.PathValue("itemId")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	if err := h.Store.DeleteItemFromCart(cartID, itemID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CartID       string `json:"cartId"`
		CustomerName string `json:"customerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CartID == "" || req.CustomerName == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}

	orderID, err := h.Store.CreateOrder(req.CartID, req.CustomerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"created": true,
		"orderId": orderID,
	})
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	order, ok := h.Store.GetOrder(orderID)
	if !ok {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}
