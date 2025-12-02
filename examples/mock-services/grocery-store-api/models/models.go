package models

import "time"

// Product represents a grocery item.
type Product struct {
	ID           int     `json:"id"`
	Category     string  `json:"category"`
	Name         string  `json:"name"`
	Manufacturer string  `json:"manufacturer,omitempty"`
	Price        float64 `json:"price,omitempty"`
	CurrentStock int     `json:"current-stock,omitempty"`
	InStock      bool    `json:"inStock"`
}

// Cart represents a shopping cart.
type Cart struct {
	CartID  string     `json:"cartId"`
	Items   []CartItem `json:"items"`
	Created time.Time  `json:"created"`
}

// CartItem represents an item in a shopping cart.
type CartItem struct {
	ID        int `json:"id"`
	ProductID int `json:"productId"`
	Quantity  int `json:"quantity"`
}

// Order represents a completed order.
type Order struct {
	ID           string     `json:"id"`
	Items        []CartItem `json:"items"`
	CustomerName string     `json:"customerName"`
	Total        float64    `json:"total"`
	ProcessedAt  time.Time  `json:"processedAt"`
}
