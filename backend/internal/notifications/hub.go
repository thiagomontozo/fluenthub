package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	db      *pgxpool.Pool
}

type envelope struct {
	UserID string          `json:"userId"`
	Event  string          `json:"event"`
	Data   json.RawMessage `json:"data"`
}

func NewHub(db *pgxpool.Pool) *Hub {
	return &Hub{clients: make(map[string]map[chan []byte]struct{}), db: db}
}
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
func (h *Hub) Publish(ctx context.Context, userID, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	wire, err := json.Marshal(envelope{UserID: userID, Event: event, Data: data})
	if err != nil {
		return fmt.Errorf("encode event envelope: %w", err)
	}
	if len(wire) > 7000 {
		return fmt.Errorf("event exceeds PostgreSQL notification limit")
	}
	_, err = h.db.Exec(ctx, `SELECT pg_notify('fluenthub_events',$1)`, string(wire))
	return err
}

func (h *Hub) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := h.listen(ctx); err != nil && ctx.Err() == nil {
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (h *Hub) listen(ctx context.Context) error {
	connection, err := h.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, `LISTEN fluenthub_events`); err != nil {
		return err
	}
	for {
		notification, err := connection.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var message envelope
		if json.Unmarshal([]byte(notification.Payload), &message) != nil {
			continue
		}
		h.dispatch(message.UserID, message.Event, message.Data)
	}
}

func (h *Hub) dispatch(userID, event string, data json.RawMessage) {
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
