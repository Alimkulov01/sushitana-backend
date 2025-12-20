package ws

import (
	"encoding/json"
	"sushitana/internal/structs"
	"sync"
	"time"
)
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{} // tgId -> set(client)
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[int64]map[*Client]struct{}),
	}
}

func (h *Hub) Register(tgId int64, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	set, ok := h.clients[tgId]
	if !ok {
		set = make(map[*Client]struct{})
		h.clients[tgId] = set
	}
	set[c] = struct{}{}
}

func (h *Hub) Unregister(tgId int64, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	set, ok := h.clients[tgId]
	if !ok {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.clients, tgId)
	}
}

func (h *Hub) BroadcastToUser(tgId int64, evt structs.Event) {
	h.mu.RLock()
	set, ok := h.clients[tgId]
	if !ok || len(set) == 0 {
		h.mu.RUnlock()
		return
	}

	evt.TS = time.Now().UTC()
	b, _ := json.Marshal(evt)

	clients := make([]*Client, 0, len(set))
	for c := range set {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.SendRaw(b)
	}
}
