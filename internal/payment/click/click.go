package click

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
	HandleCompleteCallback(ctx context.Context, body io.Reader) error
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

func (s *service) CheckoutPrepare(ctx context.Context, req structs.CheckoutPrepareRequest) (structs.CheckoutPrepareResponse, error) {
	url := "https://api.click.uz/v2/internal/checkout/prepare"
	jsonData := utils.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
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

	var result structs.CheckoutPrepareResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		s.logger.Error(ctx, "Click CheckoutPrepare: failed to decode response", zap.Error(err))
		return structs.CheckoutPrepareResponse{}, err
	}

	if httpResp.StatusCode != http.StatusOK {
		s.logger.Warn(ctx, "Click CheckoutPrepare: non-200 status", zap.Int("status", httpResp.StatusCode), zap.Any("resp", result))
		return result, errors.New(result.ErrorNote)
	}

	return result, nil
}

func (s *service) CheckoutInvoice(ctx context.Context, req structs.CheckoutInvoiceRequest) (structs.CheckoutInvoiceResponse, error) {
	url := "https://api.click.uz/v2/internal/checkout/invoice"

	jsonData, err := json.Marshal(req)
	if err != nil {
		s.logger.Error(ctx, "CheckoutInvoice: marshal failed", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		s.logger.Error(ctx, "CheckoutInvoice: create http request failed", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "CheckoutInvoice: http request failed", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}
	defer httpResp.Body.Close()

	var result structs.CheckoutInvoiceResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		s.logger.Error(ctx, "CheckoutInvoice: decode failed", zap.Error(err))
		return structs.CheckoutInvoiceResponse{}, err
	}

	if httpResp.StatusCode != http.StatusOK {
		s.logger.Warn(ctx, "CheckoutInvoice: non-200 status", zap.Int("status", httpResp.StatusCode), zap.Any("resp", result))
		return result, errors.New(result.ErrorNote)
	}

	// Agar invoice yaratildi va siz invoices jadvaliga yozmoqchi bo'lsangiz:
	// repo.CreateInvoice(ctx, invoice) chaqiruvi qo'yilishi mumkin — quyida repo interfeys qismi izohlangan.
	return result, nil
}

// Retrieve — Click API orqali request_id yoki transaction_id bo'yicha ma'lumot olish.
// Eslatma: Click retrieve endpoint metodi (GET/POST) va parametr nomlarini Click hujjatlariga moslang.
func (s *service) Retrieve(ctx context.Context, requestId string) (structs.RetrieveResponse, error) {
	url := "https://api.click.uz/v2/internal/checkout/retrieve"

	payload := map[string]string{
		"request_id": requestId,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error(ctx, "Retrieve: marshal payload failed", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		s.logger.Error(ctx, "Retrieve: create http request failed", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "Retrieve: http request failed", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}
	defer httpResp.Body.Close()

	var result structs.RetrieveResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		s.logger.Error(ctx, "Retrieve: decode failed", zap.Error(err))
		return structs.RetrieveResponse{}, err
	}

	if httpResp.StatusCode != http.StatusOK {
		s.logger.Warn(ctx, "Retrieve: non-200 status", zap.Int("status", httpResp.StatusCode), zap.Any("resp", result))
		// qaytadigan ma'lumotga asosan xatolikni qaytaring
		return result, errors.New("retrieve failed")
	}

	return result, nil
}

// HandleCompleteCallback — Click tomonidan kelgan `complete` callback body sini o'qiydi, parse qiladi va repo orqali invoice/order holatini yangilaydi.
// NOTE: Click hujjatlaridagi signature / token tekshiruvini shu yerga qo'shish zarur (masalan header yoki body ichidagi sign).
func (s *service) HandleCompleteCallback(ctx context.Context, body io.Reader) error {
	// Click complete callback modelini structs ga moslab yozing — misol uchun quyidagicha:
	var cb struct {
		ErrorCode    int64  `json:"error_code"`
		ErrorNote    string `json:"error_note"`
		RequestId    string `json:"request_id"`
		Amount       int64  `json:"amount"`
		ClickTransID int64  `json:"click_trans_id"`
		// boshqa maydonlar...
	}

	if err := json.NewDecoder(body).Decode(&cb); err != nil {
		s.logger.Error(ctx, "HandleCompleteCallback: decode failed", zap.Error(err))
		return err
	}

	// TODO: signature tekshirish — Click docs ga ko'ra bu yerda verify qiling (masalan secret + sign param).
	// if !verifySignature(...) { return errors.New("invalid signature") }

	if cb.ErrorCode == 0 {
		if err := s.clickrepo.UpdateStatus(ctx, cb.RequestId, "PAID"); err != nil {
			s.logger.Error(ctx, "HandleCompleteCallback: repo update failed", zap.Error(err))
			return err
		}
	} else {
		// xatolik bo'lsa statusni FAILED yoki boshqa holatga o'zgartirish
		if err := s.clickrepo.UpdateStatus(ctx, cb.RequestId, "UNPAID"); err != nil {
			s.logger.Error(ctx, "HandleCompleteCallback: repo update failed", zap.Error(err))
			return err
		}
	}

	return nil
}
