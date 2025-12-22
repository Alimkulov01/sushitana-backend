package client

import (
	"errors"
	"net/http"
	"strings"
	"time"

	client "sushitana/internal/client"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"
	"sushitana/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Handler interface {
		CreateClient(c *gin.Context)
		GetListClient(c *gin.Context)
		GetByIDClient(c *gin.Context)
		DeleteClient(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger        logger.Logger
		ClientService client.Service
	}

	handler struct {
		logger        logger.Logger
		clientService client.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:        p.Logger,
		clientService: p.ClientService,
	}
}

func (h *handler) CreateClient(c *gin.Context) {
	var (
		response structs.Response
		request  structs.CreateClient
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	client, err := h.clientService.Create(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			response = responses.BadRequest
			return
		}
		h.logger.Error(ctx, " err on h.clientService.Create", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = client.ID
}

func (h *handler) GetByIDClient(c *gin.Context) {
	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	respond, err := h.clientService.GetByTgID(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.clientService.GetByID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = respond
}

func (h *handler) GetListClient(c *gin.Context) {
	var (
		response structs.Response
		filter   structs.GetListClientRequest
		ctx      = c.Request.Context()

		phoneNumber      = c.Query("phone_number")
		name             = c.Query("name")
		offset           = c.Query("offset")
		limit            = c.Query("limit")
		createdAtFromStr = c.Query("created_at_from")
		createdAtToStr   = c.Query("created_at_to")
	)

	filter.PhoneNumber = phoneNumber
	filter.Name = name
	filter.Limit = int64(utils.StrToInt(limit))
	filter.Offset = int64(utils.StrToInt(offset))
	if strings.TrimSpace(createdAtFromStr) != "" {
		t, err := time.Parse(time.RFC3339Nano, createdAtFromStr)
		if err != nil {
			response = responses.BadRequest
			response.Message = "invalid created_at_from (RFC3339 expected)"
			defer reply.Json(c.Writer, http.StatusBadRequest, &response)
			return
		}
		filter.CreatedAtFrom = &t
	}

	if strings.TrimSpace(createdAtToStr) != "" {
		t, err := time.Parse(time.RFC3339Nano, createdAtToStr)
		if err != nil {
			response = responses.BadRequest
			response.Message = "invalid created_at_to (RFC3339 expected)"
			defer reply.Json(c.Writer, http.StatusBadRequest, &response)
			return
		}
		filter.CreatedAtTo = &t
	}

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.clientService.GetList(c, filter)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.clientService.GetList", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) DeleteClient(c *gin.Context) {

	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	err := h.clientService.Delete(c, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.clientService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}
