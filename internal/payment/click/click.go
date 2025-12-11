package click

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	clickrepo "sushitana/pkg/repository/postgres/payment_repo/click_repo"
	"sushitana/pkg/utils"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		Logger    logger.Logger
		ClickRepo clickrepo.Repo
	}

	Service interface {
		CheckoutPrepare(ctx context.Context, req structs.CheckoutPrepareRequest) (resp structs.CheckoutPrepareResponse, err error)
	}

	service struct {
		logger    logger.Logger
		clickrepo clickrepo.Repo
	}
)

func New(p Params) Service {
	return &service{
		logger:    p.Logger,
		clickrepo: p.ClickRepo,
	}
}

func (s *service) CheckoutPrepare(ctx context.Context, req structs.CheckoutPrepareRequest) (resp structs.CheckoutPrepareResponse, err error) {
	url := "https://api.click.uz/v2/internal/checkout/prepare"
	jsonData := utils.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		s.logger.Error(ctx, "Failed to create HTTP request: %v", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "HTTP request failed: %v", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}
	defer httpResp.Body.Close()

	var result structs.CheckoutPrepareResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		s.logger.Error(ctx, "Failed to decode response: %v", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}

	return result, nil

}
