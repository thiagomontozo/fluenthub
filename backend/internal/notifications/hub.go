package notifications

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Notification struct {
	ID, UserID, Type, Title, Message string
	ResourceType, ResourceID         *string
	ReadAt                           *time.Time
	CreatedAt                        time.Time
}
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[chan []byte]struct{}
}

func NewHub() *Hub { return &Hub{clients: make(map[string]map[chan []byte]struct{})} }
func (h *Hub) Subscribe(userID string) (<-chan []byte, func()) {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan []byte]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() { h.mu.Lock(); delete(h.clients[userID], ch); close(ch); h.mu.Unlock() }
}
func (h *Hub) Publish(userID, event string, payload any) {
	data, _ := json.Marshal(payload)
	message := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients[userID] {
		select {
		case ch <- message:
		default:
		}
	}
}
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, clients := range h.clients {
		for ch := range clients {
			close(ch)
		}
	}
	h.clients = make(map[string]map[chan []byte]struct{})
}
