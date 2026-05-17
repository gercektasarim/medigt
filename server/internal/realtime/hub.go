package realtime

// WebSocket fan-out hub. Server-side heartbeat (30s) lets clients detect
// half-open connections — Caliptic's P0 cache-staleness bug came from
// browsers not exposing ping/pong frames to JS, so we send app-level
// heartbeat messages and clients track lastMessageTime.

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

const heartbeatInterval = 30 * time.Second

type Scope string

const (
	ScopeOrganization Scope = "organization"
	ScopeBranch       Scope = "branch"
	ScopeUser         Scope = "user"
	ScopePatient      Scope = "patient"
	ScopeBedMap       Scope = "bed_map"
	ScopeCashRegister Scope = "cash_register"
	ScopeLabOrder     Scope = "lab_order"
)

type Event struct {
	Type    string         `json:"type"`
	Scope   Scope          `json:"scope"`
	ScopeID string         `json:"scope_id"`
	Payload map[string]any `json:"payload,omitempty"`
	TS      int64          `json:"ts"`
}

type subscription struct {
	scope   Scope
	scopeID string
}

type Client struct {
	id         string
	send       chan []byte
	subs       map[subscription]struct{}
	subsMu     sync.RWMutex
}

func newClient(id string) *Client {
	return &Client{
		id:   id,
		send: make(chan []byte, 64),
		subs: make(map[subscription]struct{}),
	}
}

func (c *Client) subscribe(s Scope, id string) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	c.subs[subscription{s, id}] = struct{}{}
}

func (c *Client) matches(s Scope, id string) bool {
	c.subsMu.RLock()
	defer c.subsMu.RUnlock()
	_, ok := c.subs[subscription{s, id}]
	return ok
}

type Hub struct {
	log        *slog.Logger
	clients    map[string]*Client
	mu         sync.RWMutex
	registerCh chan *Client
	removeCh   chan string
	broadcast  chan Event
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		log:        log,
		clients:    make(map[string]*Client),
		registerCh: make(chan *Client, 32),
		removeCh:   make(chan string, 32),
		broadcast:  make(chan Event, 256),
	}
}

func (h *Hub) Run(ctx context.Context) {
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.registerCh:
			h.mu.Lock()
			h.clients[c.id] = c
			h.mu.Unlock()
		case id := <-h.removeCh:
			h.mu.Lock()
			if c, ok := h.clients[id]; ok {
				close(c.send)
				delete(h.clients, id)
			}
			h.mu.Unlock()
		case ev := <-h.broadcast:
			payload, err := json.Marshal(ev)
			if err != nil {
				h.log.Warn("event marshal failed", "err", err)
				continue
			}
			h.mu.RLock()
			for _, c := range h.clients {
				if !c.matches(ev.Scope, ev.ScopeID) {
					continue
				}
				select {
				case c.send <- payload:
				default:
					h.log.Warn("client send buffer full, dropping", "client", c.id)
				}
			}
			h.mu.RUnlock()
		case <-heartbeat.C:
			beat, _ := json.Marshal(map[string]any{
				"type": "heartbeat",
				"ts":   time.Now().Unix(),
			})
			h.mu.RLock()
			for _, c := range h.clients {
				select {
				case c.send <- beat:
				default:
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Publish(ev Event) {
	if ev.TS == 0 {
		ev.TS = time.Now().Unix()
	}
	select {
	case h.broadcast <- ev:
	default:
		h.log.Warn("broadcast buffer full, dropping event", "type", ev.Type)
	}
}
