package click

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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

// Params — FX injection uchun parametrlar
type Params struct {
	fx.In
	Logger    logger.Logger
	ClickRepo clickrepo.Repo
}

type Service interface {
	CheckoutPrepare(ctx context.Context, req structs.CheckoutPrepareRequest) (structs.CheckoutPrepareResponse, error)
	CheckoutInvoice(ctx context.Context, req structs.CheckoutInvoiceRequest) (structs.CheckoutInvoiceResponse, error)
	Retrieve(ctx context.Context, requestId string) (structs.RetrieveResponse, error)
	CreateClickInvoice(ctx context.Context, req structs.CreateInvoiceRequest) (structs.CreateInvoiceResponse, error)

	ShopPrepare(ctx context.Context, req structs.ClickPrepareRequest) (structs.ClickPrepareResponse, error)
	ShopComplete(ctx context.Context, req structs.ClickCompleteRequest) (structs.ClickCompleteResponse, error)
}

type service struct {
	logger    logger.Logger
	clickrepo clickrepo.Repo
	client    *http.Client
}

func New(p Params) Service {
	return &service{
		logger:    p.Logger,
		clickrepo: p.ClickRepo,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}
func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func normalizeAmount(s string) string {
	return strings.TrimSpace(s)
}

func (s *service) validatePrepareSign(req structs.ClickPrepareRequest, secret string) bool {
	raw := fmt.Sprintf("%d%d%s%s%s%d%s",
		req.ClickTransId,
		req.ServiceId,
		secret,
		req.MerchantTransId,
		normalizeAmount(req.Amount),
		req.Action,
		req.SignTime,
	)
	return strings.EqualFold(md5hex(raw), req.SignString)
}

func (s *service) validateCompleteSign(req structs.ClickCompleteRequest, secret string) bool {
	raw := fmt.Sprintf("%d%d%s%s%d%s%d%s",
		req.ClickTransId,
		req.ServiceId,
		secret,
		req.MerchantTransId,
		req.MerchantPrepareId,
		normalizeAmount(req.Amount),
		req.Action,
		req.SignTime,
	)
	return strings.EqualFold(md5hex(raw), req.SignString)
}

func (s *service) ShopPrepare(ctx context.Context, req structs.ClickPrepareRequest) (structs.ClickPrepareResponse, error) {
	if req.Action != nil {
		return structs.ClickPrepareResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -3,
			ErrorNote:       "Action not found",
		}, nil
	}

	secret := os.Getenv("CLICK_SECRET_KEY")
	if secret == "" {
		s.logger.Error(ctx, "CLICK_SECRET_KEY is empty")
		return structs.ClickPrepareResponse{Error: -8, ErrorNote: "Server config error"}, errors.New("CLICK_SECRET_KEY empty")
	}

	if !s.validatePrepareSign(req, secret) {
		return structs.ClickPrepareResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -1,
			ErrorNote:       "SIGN CHECK FAILED!",
		}, nil
	}

	inv, err := s.clickrepo.GetByMerchantTransID(ctx, req.MerchantTransId)
	if err != nil {
		return structs.ClickPrepareResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -5,
			ErrorNote:       "Invoice not found",
		}, nil
	}

	if inv.Amount != req.Amount {
		return structs.ClickPrepareResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -2,
			ErrorNote:       "The total payment value is not equal.",
		}, nil
	}
	mpid, err := s.clickrepo.UpsertPrepare(ctx, req.MerchantTransId, req.ClickTransId, req.ClickPaydocId, req.Amount)
	if err != nil {
		s.logger.Error(ctx, "UpsertPrepare failed", zap.Error(err))
		return structs.ClickPrepareResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -7,
			ErrorNote:       "Failed to update invoice",
		}, nil
	}

	return structs.ClickPrepareResponse{
		ClickTransId:      req.ClickTransId,
		MerchantTransId:   req.MerchantTransId,
		MerchantPrepareId: mpid,
		Error:             0,
		ErrorNote:         "Success",
	}, nil
}

func (s *service) ShopComplete(ctx context.Context, req structs.ClickCompleteRequest) (structs.ClickCompleteResponse, error) {
	if req.Action != 1 {
		return structs.ClickCompleteResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -3,
			ErrorNote:       "Action not found",
		}, nil
	}

	secret := os.Getenv("CLICK_SECRET_KEY")
	if secret == "" {
		s.logger.Error(ctx, "CLICK_SECRET_KEY is empty")
		return structs.ClickCompleteResponse{Error: -8, ErrorNote: "Server config error"}, errors.New("CLICK_SECRET_KEY empty")
	}

	if !s.validateCompleteSign(req, secret) {
		return structs.ClickCompleteResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -1,
			ErrorNote:       "SIGN CHECK FAILED!",
		}, nil
	}

	status := "PAID"
	if req.Error != 0 {
		status = "FAILED"
	}

	invoiceID, orderID, err := s.clickrepo.UpdateOnComplete(ctx, req.MerchantTransId, req.MerchantPrepareId, req.ClickTransId, status)
	if err != nil {
		s.logger.Error(ctx, "UpdateOnComplete failed", zap.Error(err))
		return structs.ClickCompleteResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -7,
			ErrorNote:       "Failed to update invoice",
		}, nil
	}

	_ = orderID
	_ = invoiceID

	return structs.ClickCompleteResponse{
		ClickTransId:      req.ClickTransId,
		MerchantTransId:   req.MerchantTransId,
		MerchantConfirmId: req.MerchantPrepareId,
		Error:             0,
		ErrorNote:         "Success",
	}, nil
}

func (s *service) CheckoutPrepare(ctx context.Context, req structs.CheckoutPrepareRequest) (structs.CheckoutPrepareResponse, error) {
	url := "https://api.click.uz/v2/internal/checkout/prepare"

	body, err := json.Marshal(req)
	if err != nil {
		s.logger.Error(ctx, "Click CheckoutPrepare: marshal failed", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		s.logger.Error(ctx, "Click CheckoutPrepare: failed to create request", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "Click CheckoutPrepare: http request failed", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}
	defer httpResp.Body.Close()

	var resp structs.CheckoutPrepareResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		s.logger.Error(ctx, "Click CheckoutPrepare: decode failed", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}

	if httpResp.StatusCode != http.StatusOK {
		s.logger.Warn(ctx, "Click CheckoutPrepare: non-200 status", zap.Int("status", httpResp.StatusCode), zap.Any("resp", resp))
		if resp.ErrorNote != "" {
			return resp, errors.New(resp.ErrorNote)
		}
		return resp, fmt.Errorf("click prepare non-200: %d", httpResp.StatusCode)
	}

	if resp.ErrorCode != 0 {
		return resp, errors.New(resp.ErrorNote)
	}

	return resp, nil
}

func (s service) CreateClickInvoice(ctx context.Context, req structs.CreateInvoiceRequest) (structs.CreateInvoiceResponse, error) {
	url := "https://api.click.uz/v2/merchant/invoice/create"

	jsonData := utils.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		s.logger.Error(ctx, "Failed to create HTTP request: %v", zap.Error(err))
		return structs.CreateInvoiceResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "HTTP request failed: %v", zap.Error(err))
		return structs.CreateInvoiceResponse{}, err
	}
	defer httpResp.Body.Close()

	var result structs.CreateInvoiceResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		s.logger.Error(ctx, "Failed to decode response: %v", zap.Error(err))
		return structs.CreateInvoiceResponse{}, err
	}

	return result, nil
}

func (s *service) CheckoutInvoice(ctx context.Context, req structs.CheckoutInvoiceRequest) (structs.CheckoutInvoiceResponse, error) {
	url := "https://api.click.uz/v2/internal/checkout/invoice"

	body, err := json.Marshal(req)
	if err != nil {
		s.logger.Error(ctx, "Click CheckoutInvoice: marshal failed", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		s.logger.Error(ctx, "Click CheckoutInvoice: failed to create request", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "Click CheckoutInvoice: http request failed", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}
	defer httpResp.Body.Close()

	var resp structs.CheckoutInvoiceResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		s.logger.Error(ctx, "Click CheckoutInvoice: decode failed", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}

	if httpResp.StatusCode != http.StatusOK {
		s.logger.Warn(ctx, "Click CheckoutInvoice: non-200 status", zap.Int("status", httpResp.StatusCode), zap.Any("resp", resp))
		if resp.ErrorNote != "" {
			return resp, errors.New(resp.ErrorNote)
		}
		return resp, fmt.Errorf("click invoice non-200: %d", httpResp.StatusCode)
	}

	if resp.ErrorCode != 0 {
		return resp, errors.New(resp.ErrorNote)
	}

	return resp, nil
}

func (s *service) Retrieve(ctx context.Context, requestId string) (structs.RetrieveResponse, error) {
	url := "https://api.click.uz/v2/internal/checkout/retrieve"

	payload := map[string]string{
		"request_id": requestId,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error(ctx, "Click Retrieve: marshal failed", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		s.logger.Error(ctx, "Click Retrieve: failed to create request", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "Click Retrieve: http request failed", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}
	defer httpResp.Body.Close()

	var resp structs.RetrieveResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		s.logger.Error(ctx, "Click Retrieve: decode failed", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}

	if httpResp.StatusCode != http.StatusOK {
		s.logger.Warn(ctx, "Click Retrieve: non-200 status", zap.Int("status", httpResp.StatusCode), zap.Any("resp", resp))
		if resp.ErrorNote != "" {
			return resp, errors.New(resp.ErrorNote)
		}
		return resp, fmt.Errorf("click retrieve non-200: %d", httpResp.StatusCode)
	}

	// Retrieve response ham error_code bilan keladi (agar structda bo'lsa)
	// if resp.ErrorCode != 0 { return resp, errors.New(resp.ErrorNote) }

	return resp, nil
}
