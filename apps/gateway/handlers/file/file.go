package file

import (
	"errors"
	"net/http"

	file "sushitana/internal/file"
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
		CreateFile(c *gin.Context)
		GetListFile(c *gin.Context)
		GetByIDFile(c *gin.Context)
		DeleteFile(c *gin.Context)
		GetImage(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger      logger.Logger
		FileService file.Service
	}

	handler struct {
		logger      logger.Logger
		fileService file.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:      p.Logger,
		fileService: p.FileService,
	}
}

func (h *handler) CreateFile(c *gin.Context) {
	var (
		response structs.Response
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	imageType := c.PostForm("image_type")
	if imageType == "" {
		h.logger.Warn(ctx, "image_type is required")
		response = responses.BadRequest
		return
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		h.logger.Warn(ctx, "file is missing", zap.Error(err))
		response = responses.BadRequest
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.logger.Error(ctx, "failed to open file", zap.Error(err))
		response = responses.InternalErr
		return
	}
	defer file.Close()

	publicURL, err := utils.UploadMultipartFile(file, fileHeader.Filename, imageType)
	if err != nil {
		h.logger.Error(ctx, "failed to upload to s3", zap.Error(err))
		response = responses.InternalErr
		return
	}

	created, err := h.fileService.Create(c, structs.CreateImage{
		ImageType: imageType,
		Image:     publicURL,
	})
	if err != nil {
		h.logger.Error(ctx, "db create error", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = created
}

func (h *handler) GetByIDFile(c *gin.Context) {
	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	respond, err := h.fileService.GetById(c, structs.ImagePrimaryKey{Id: id})
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.fileService.GetById", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = respond
}

func (h *handler) GetListFile(c *gin.Context) {
	var (
		response structs.Response
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.fileService.GetAll(c)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.fileService.GetAll", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) GetImage(c *gin.Context) {
	var (
		response structs.Response
		request  structs.GetImageRequest
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	list, err := h.fileService.GetImage(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.fileService.GetImage", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = list
}

func (h *handler) DeleteFile(c *gin.Context) {

	var (
		response structs.Response
		idStr    = c.Param("id")
		ctx      = c.Request.Context()
	)
	defer reply.Json(c.Writer, http.StatusOK, &response)
	id := cast.ToInt64(idStr)
	err := h.fileService.Delete(c, structs.ImagePrimaryKey{Id: id})
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.fileService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}
