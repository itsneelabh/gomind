package store

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"grocery-store-api/models"

	"github.com/google/uuid"
)

// Store holds the in-memory data.
type Store struct {
	mu       sync.RWMutex
	Products map[int]models.Product
	Carts    map[string]models.Cart
	Orders   map[string]models.Order
}

// NewStore initializes a new Store.
func NewStore() *Store {
	return &Store{
		Products: make(map[int]models.Product),
		Carts:    make(map[string]models.Cart),
		Orders:   make(map[string]models.Order),
	}
}

// Seed populates the store with initial data.
func (s *Store) Seed() {
	s.mu.Lock()
	defer s.mu.Unlock()

	categories := []string{"coffee", "fresh-produce", "meat-seafood", "candy", "dairy"}

	// Seed 20 products
	for i := 1; i <= 20; i++ {
		category := categories[rand.Intn(len(categories))]
		product := models.Product{
			ID:           4640 + i, // Starting ID similar to example
			Category:     category,
			Name:         fmt.Sprintf("%s Product %d", category, i),
			Manufacturer: "Generic Brand",
			Price:        float64(rand.Intn(5000)) / 100.0,
			CurrentStock: rand.Intn(50),
			InStock:      true,
		}
		s.Products[product.ID] = product
	}
}

// GetProducts returns a list of products, optionally filtered by category.
func (s *Store) GetProducts(category string, limit int) []models.Product {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.Product
	count := 0
	for _, p := range s.Products {
		if category != "" && p.Category != category {
			continue
		}
		result = append(result, p)
		count++
		if limit > 0 && count >= limit {
			break
		}
	}
	return result
}

// GetProduct returns a product by ID.
func (s *Store) GetProduct(id int) (models.Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.Products[id]
	return p, ok
}

// CreateCart creates a new cart.
func (s *Store) CreateCart() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	cartID := uuid.New().String()
	s.Carts[cartID] = models.Cart{
		CartID:  cartID,
		Items:   []models.CartItem{},
		Created: time.Now(),
	}
	return cartID
}

// GetCart returns a cart by ID.
func (s *Store) GetCart(cartID string) (models.Cart, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.Carts[cartID]
	return c, ok
}

// AddItemToCart adds an item to a cart.
func (s *Store) AddItemToCart(cartID string, productID, quantity int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, ok := s.Carts[cartID]
	if !ok {
		return 0, fmt.Errorf("cart not found")
	}

	// Check if product exists
	if _, ok := s.Products[productID]; !ok {
		return 0, fmt.Errorf("product not found")
	}

	itemID := rand.Intn(100000) // Simple random ID for item
	item := models.CartItem{
		ID:        itemID,
		ProductID: productID,
		Quantity:  quantity,
	}
	cart.Items = append(cart.Items, item)
	s.Carts[cartID] = cart
	return itemID, nil
}

// UpdateItemInCart updates the quantity of an item in a cart.
func (s *Store) UpdateItemInCart(cartID string, itemID, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, ok := s.Carts[cartID]
	if !ok {
		return fmt.Errorf("cart not found")
	}

	found := false
	for i, item := range cart.Items {
		if item.ID == itemID {
			cart.Items[i].Quantity = quantity
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("item not found in cart")
	}
	s.Carts[cartID] = cart
	return nil
}

// DeleteItemFromCart removes an item from a cart.
func (s *Store) DeleteItemFromCart(cartID string, itemID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, ok := s.Carts[cartID]
	if !ok {
		return fmt.Errorf("cart not found")
	}

	newItems := []models.CartItem{}
	found := false
	for _, item := range cart.Items {
		if item.ID == itemID {
			found = true
			continue
		}
		newItems = append(newItems, item)
	}
	if !found {
		return fmt.Errorf("item not found in cart")
	}
	cart.Items = newItems
	s.Carts[cartID] = cart
	return nil
}

// CreateOrder creates a new order from a cart.
func (s *Store) CreateOrder(cartID, customerName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, ok := s.Carts[cartID]
	if !ok {
		return "", fmt.Errorf("cart not found")
	}

	var total float64
	for _, item := range cart.Items {
		if p, ok := s.Products[item.ProductID]; ok {
			total += p.Price * float64(item.Quantity)
		}
	}

	orderID := "order_" + uuid.New().String()[:8]
	order := models.Order{
		ID:           orderID,
		Items:        cart.Items,
		CustomerName: customerName,
		Total:        total,
		ProcessedAt:  time.Now(),
	}
	s.Orders[orderID] = order
	return orderID, nil
}

// GetOrder returns an order by ID.
func (s *Store) GetOrder(orderID string) (models.Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.Orders[orderID]
	return o, ok
}
