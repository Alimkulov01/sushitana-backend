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
	admins  map[*Client]struct{}
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

// admin

func (h *Hub) RegisterAdmin(c *Client) {
	h.mu.Lock()
	h.admins[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) UnregisterAdmin(c *Client) {
	h.mu.Lock()
	delete(h.admins, c)
	h.mu.Unlock()
}

func (h *Hub) BroadcastToAdmins(evt structs.Event) {
	evt.TS = time.Now().UTC()

	h.mu.Lock()
	if len(h.admins) == 0 {
		h.mu.Unlock()
		return
	}

	clients := make([]*Client, 0, len(h.admins))
	for c := range h.admins {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	b, err := json.Marshal(evt)
	if err != nil {
		return
	}
	for _, c := range clients {
		c.SendRaw(b)
	}
}

func (h *Hub) BroadcastToAdminsAndUser(tgId int64, evt structs.Event) {
	h.BroadcastToAdmins(evt)
	h.BroadcastToUser(tgId, evt)
}
