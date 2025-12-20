package ws

import (
	"net/http"
	"strconv"

	rtws "sushitana/internal/ws" // ✅ internal/ws Hub shu yerda

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/fx"
)

var (
	Module = fx.Provide(New)
)

type (
	Handler interface {
		OrdersWS(c *gin.Context)
	}

	Params struct {
		fx.In
		Hub *rtws.Hub // ✅ mana shu joy sabab “undefined: Hub” chiqyapti
	}

	handler struct {
		hub *rtws.Hub
	}
)

func New(p Params) Handler {
	return &handler{
		hub: p.Hub,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // prod’da cheklang
}

// GET /api/v1/ws/orders?tg_id=8599592433
func (h *handler) OrdersWS(c *gin.Context) {
	tgIDStr := c.Query("tg_id")
	if tgIDStr == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "tg_id is required"})
		return
	}

	tgId, err := strconv.ParseInt(tgIDStr, 10, 64)
	if err != nil || tgId <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tg_id"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// ✅ Client ham internal/ws ichida bo‘lsa shu tarzda chaqiring
	client := rtws.NewClient(tgId, conn, h.hub)
	h.hub.Register(tgId, client)
	client.Run()
}
