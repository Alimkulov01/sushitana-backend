package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
	maxMsgSize = 16 * 1024
)

type ClientKind string

const (
	KindUser  ClientKind = "user"
	KindAdmin ClientKind = "admin"
)

type Client struct {
	kind ClientKind
	tgId int64
	conn *websocket.Conn
	hub  *Hub
	send chan []byte
}

func NewClient(tgId int64, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		kind: KindUser,
		tgId: tgId,
		conn: conn,
		hub:  hub,
		send: make(chan []byte, 256),
	}
}
func NewAdminClient(conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		kind: KindAdmin,
		conn: conn,
		hub:  hub,
		send: make(chan []byte, 256),
	}
}

func (c *Client) SendRaw(b []byte) {
	select {
	case c.send <- b:
	default:
		// queue to‘lib qolsa – connectionni yopamiz (memory leak bo‘lmasin)
		_ = c.conn.Close()
	}
}

func (c *Client) readPump() {
	defer func() {
		if c.kind == KindAdmin {
			c.hub.UnregisterAdmin(c)
		} else {
			c.hub.Unregister(c.tgId, c)
		}
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		// biz hozir clientdan msg qabul qilmaymiz (faqat read qilib connection tirikligini saqlaymiz)
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}
