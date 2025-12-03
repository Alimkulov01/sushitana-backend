package menu

import (
	"errors"
	"net/http"
	"sushitana/internal/menu"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Handler interface {
		GetMenu(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger      logger.Logger
		MenuService menu.Service
	}

	handler struct {
		logger      logger.Logger
		menuService menu.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:      p.Logger,
		menuService: p.MenuService,
	}
}

func (h *handler) GetMenu(c *gin.Context) {
	var (
		response structs.Response
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	resp, err := h.menuService.GetMenu(c)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.menuService.GetMenu", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = resp
}
