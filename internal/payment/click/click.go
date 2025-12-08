package click

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
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
		Logger logger.Logger
	}

	Service interface {
		// 1. Invoice yaratish (sen -> Click)
		CreateClickInvoice(ctx context.Context, req structs.CreateInvoiceRequest) (structs.CreateInvoiceResponse, error)
	}

	service struct {
		logger logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		logger: p.Logger,
	}
}

func (s service) CreateClickInvoice(ctx context.Context, req structs.CreateInvoiceRequest) (structs.CreateInvoiceResponse, error) {
	url := "https://api.click.uz/v2/merchant/invoice/create"

	jsonData := utils.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		s.logger.Error(ctx, "Failed to create HTTP request", zap.Error(err))
		return structs.CreateInvoiceResponse{}, err
	}

	merchantUserID := os.Getenv("CLICK_MERCHANT_USER_ID")
	merchantID := os.Getenv("CLICK_MERCHANT_ID")
	secretKey := os.Getenv("CLICK_SECRET_KEY")
	serviceID := os.Getenv("CLICK_SERVICE_ID")
	paymentBaseURL := os.Getenv("CLICK_PAYMENT_BASE_URL")

	authHeader, _ := utils.ClickAuthHeader(merchantUserID, secretKey)

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Auth", authHeader)

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "HTTP request failed", zap.Error(err))
		return structs.CreateInvoiceResponse{}, err
	}
	defer httpResp.Body.Close()

	var result structs.CreateInvoiceResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		s.logger.Error(ctx, "Failed to decode response", zap.Error(err))
		return structs.CreateInvoiceResponse{}, err
	}

	if result.ErrorCode != 0 {
		s.logger.Error(ctx, "click invoice error",
			zap.Int64("error_code", result.ErrorCode),
			zap.String("error_note", result.ErrorNote),
		)
		return result, fmt.Errorf("click invoice error: %d - %s", result.ErrorCode, result.ErrorNote)
	}

	// Pay URL
	if paymentBaseURL != "" && serviceID != "" && merchantID != "" && result.InvoiceId != 0 {
		result.PaymentUrl = fmt.Sprintf(
			"%s?service_id=%s&merchant_id=%s&amount=%f&transaction_param=%s",
			paymentBaseURL,
			serviceID,
			merchantID,
			req.Amount,                // TODO: CreateInvoiceRequest ichidan o‘qish
			req.MerchantTransactionId, // order ID
		)
	}

	return result, nil
}
