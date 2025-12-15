package click

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	clickrepo "sushitana/pkg/repository/postgres/payment_repo/click_repo"

	"github.com/spf13/cast"
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
	InvoiceStatus(ctx context.Context, serviceID int64, invoiceID int64) (structs.ClickInvoiceStatusResponse, error)

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
	if req.Action == nil {
		return false
	}
	raw := fmt.Sprintf("%d%d%s%s%s%d%s",
		req.ClickTransId,
		req.ServiceId,
		secret,
		req.MerchantTransId,
		normalizeAmount(req.Amount),
		*req.Action, // <-- DEREF
		req.SignTime,
	)
	return strings.EqualFold(md5hex(raw), req.SignString)
}

func (s *service) validateCompleteSign(req structs.ClickCompleteRequest, secret string) bool {
	if req.Action == nil {
		return false
	}
	raw := fmt.Sprintf("%d%d%s%s%d%s%d%s",
		req.ClickTransId,
		req.ServiceId,
		secret,
		req.MerchantTransId,
		req.MerchantPrepareId,
		normalizeAmount(req.Amount),
		*req.Action, // <-- FIX: deref
		req.SignTime,
	)
	return strings.EqualFold(md5hex(raw), req.SignString)
}
func (s *service) ShopPrepare(ctx context.Context, req structs.ClickPrepareRequest) (structs.ClickPrepareResponse, error) {
	if req.Action == nil || *req.Action != 0 {
		return structs.ClickPrepareResponse{
			ClickTransId: req.ClickTransId,
			Error:        -3,
			ErrorNote:    "Action not found",
		}, nil
	}

	secret := os.Getenv("CLICK_SECRET_KEY")
	if secret == "" {
		s.logger.Error(ctx, "CLICK_SECRET_KEY is empty")
		return structs.ClickPrepareResponse{Error: -8, ErrorNote: "Server config error"}, errors.New("CLICK_SECRET_KEY empty")
	}

	if !s.validatePrepareSign(req, secret) {
		return structs.ClickPrepareResponse{
			ClickTransId: req.ClickTransId,
			Error:        -1,
			ErrorNote:    "SIGN CHECK FAILED!",
		}, nil
	}

	if strings.TrimSpace(req.MerchantTransId) == "" {
		s.logger.Warn(ctx, "click prepare without merchant_trans_id (invoice/SMS flow)",
			zap.Int64("click_trans_id", req.ClickTransId),
			zap.Int64("click_paydoc_id", req.ClickPaydocId),
			zap.String("amount", req.Amount),
		)
		return structs.ClickPrepareResponse{
			ClickTransId:      req.ClickTransId,
			MerchantTransId:   "",
			MerchantPrepareId: req.ClickPaydocId, // stabil qiymat
			Error:             0,
			ErrorNote:         "Success",
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

	reqAmt := math.Round(cast.ToFloat64(req.Amount)*100) / 100
	invAmt := math.Round(cast.ToFloat64(inv.Amount)*100) / 100

	if reqAmt != invAmt {
		return structs.ClickPrepareResponse{
			ClickTransId:      req.ClickTransId,
			MerchantTransId:   req.MerchantTransId,
			MerchantPrepareId: inv.MerchantPrepareID,
			Error:             -2,
			ErrorNote:         "Incorrect amount",
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
	if req.Action == nil || *req.Action != 1 {
		return structs.ClickCompleteResponse{
			ClickTransId: req.ClickTransId,
			Error:        -3,
			ErrorNote:    "Action not found",
		}, nil
	}

	secret := os.Getenv("CLICK_SECRET_KEY")
	if secret == "" {
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

	// SMS(invoice) flow: merchant_trans_id bo‘sh kelsa ham to‘lovni sindirmaymiz
	if strings.TrimSpace(req.MerchantTransId) == "" {
		s.logger.Warn(ctx, "click complete without merchant_trans_id (invoice/SMS flow)",
			zap.Int64("click_trans_id", req.ClickTransId),
			zap.Int64("merchant_prepare_id", req.MerchantPrepareId),
			zap.String("amount", req.Amount),
		)
		return structs.ClickCompleteResponse{
			ClickTransId:      req.ClickTransId,
			MerchantTransId:   "",
			MerchantConfirmId: req.MerchantPrepareId,
			Error:             0,
			ErrorNote:         "Success",
		}, nil
	}

	// Shop flow (mti bor) => eski repo update
	status := "PAID"
	if req.Error != nil && *req.Error != 0 {
		status = "FAILED"
	}

	_, _, err := s.clickrepo.UpdateOnComplete(ctx, req.MerchantTransId, req.MerchantPrepareId, req.ClickTransId, status)
	if err != nil {
		return structs.ClickCompleteResponse{
			ClickTransId:    req.ClickTransId,
			MerchantTransId: req.MerchantTransId,
			Error:           -7,
			ErrorNote:       "Failed to update invoice",
		}, nil
	}

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

func (s *service) CreateClickInvoice(ctx context.Context, req structs.CreateInvoiceRequest) (structs.CreateInvoiceResponse, error) {
	url := "https://api.click.uz/v2/merchant/invoice/create"

	merchantUserID := os.Getenv("CLICK_MERCHANT_USER_ID") // docs: merchant_user_id kerak :contentReference[oaicite:5]{index=5}
	secretKey := os.Getenv("CLICK_SECRET_KEY")

	ts := time.Now().Unix()
	sum := sha1.Sum([]byte(fmt.Sprintf("%d%s", ts, secretKey))) // sha1(timestamp + secret_key) :contentReference[oaicite:6]{index=6}
	digest := hex.EncodeToString(sum[:])
	auth := fmt.Sprintf("%s:%s:%d", merchantUserID, digest, ts)

	body, err := json.Marshal(req)
	if err != nil {
		return structs.CreateInvoiceResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return structs.CreateInvoiceResponse{}, err
	}
	fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>", httpReq)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Auth", auth)
	s.logger.Info(ctx, "click invoice/create outgoing",
		zap.ByteString("body", body),
		zap.String("auth", auth),
	)
	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		return structs.CreateInvoiceResponse{}, err
	}
	defer httpResp.Body.Close()

	var result structs.CreateInvoiceResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return structs.CreateInvoiceResponse{}, err
	}

	if httpResp.StatusCode != http.StatusOK || result.ErrorCode != 0 {
		return result, fmt.Errorf("click invoice/create failed: status=%d err=%d note=%s",
			httpResp.StatusCode, result.ErrorCode, result.ErrorNote)
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

func (s *service) buildAuth() (string, error) {
	merchantUserID := os.Getenv("CLICK_MERCHANT_USER_ID")
	secretKey := os.Getenv("CLICK_SECRET_KEY")
	if merchantUserID == "" || secretKey == "" {
		return "", fmt.Errorf("CLICK_MERCHANT_USER_ID yoki CLICK_SECRET_KEY empty")
	}

	ts := time.Now().Unix()
	sum := sha1.Sum([]byte(fmt.Sprintf("%d%s", ts, secretKey)))
	digest := hex.EncodeToString(sum[:])

	return fmt.Sprintf("%s:%s:%d", merchantUserID, digest, ts), nil
}

func (s *service) InvoiceStatus(ctx context.Context, serviceID int64, invoiceID int64) (structs.ClickInvoiceStatusResponse, error) {
	auth, err := s.buildAuth()
	if err != nil {
		return structs.ClickInvoiceStatusResponse{}, err
	}

	url := fmt.Sprintf("https://api.click.uz/v2/merchant/invoice/status/%d/%d", serviceID, invoiceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return structs.ClickInvoiceStatusResponse{}, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Auth", auth)

	resp, err := s.client.Do(req)
	if err != nil {
		return structs.ClickInvoiceStatusResponse{}, err
	}
	defer resp.Body.Close()

	var out structs.ClickInvoiceStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return structs.ClickInvoiceStatusResponse{}, err
	}

	if resp.StatusCode != http.StatusOK || out.ErrorCode != 0 {
		return out, fmt.Errorf("invoice/status failed: http=%d err=%d note=%s",
			resp.StatusCode, out.ErrorCode, out.ErrorNote)
	}

	return out, nil
}
