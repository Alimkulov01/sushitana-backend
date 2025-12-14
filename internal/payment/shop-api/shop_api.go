package shopapi

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	clickrepo "sushitana/pkg/repository/postgres/payment_repo/click_repo"
	"time"

	"go.uber.org/fx"
)

var (
	Module = fx.Provide(New)
)

type Params struct {
	fx.In
	Logger    logger.Logger
	ClickRepo clickrepo.Repo
}

type Service interface {
	BuildPayURL(ctx context.Context, req structs.PayLinkParams) (string, error)

	PaymentStatus(ctx context.Context, serviceID int64, paymentID int64) (structs.PaymentStatusResponse, error)
	PaymentStatusByMTI(ctx context.Context, serviceID int64, merchantTransID string, paymentDate time.Time) (structs.StatusByMTIResponse, error)
	PaymentReversal(ctx context.Context, serviceID int64, paymentID int64) (structs.ReversalResponse, error)
}

type service struct {
	logger    logger.Logger
	clickrepo clickrepo.Repo
	client    *http.Client

	defaultServiceID  string
	defaultMerchantID string
	merchantCfg       structs.MerchantConfig
}

type UsecaseParams struct {
	fx.In
	Logger    logger.Logger
	OrderRepo orderrepo.Repo
	ClickRepo clickrepo.Repo
}

func New(p Params) Service {
	return &service{
		logger:    p.Logger,
		clickrepo: p.ClickRepo,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		defaultServiceID:  strings.TrimSpace(os.Getenv("CLICK_SERVICE_ID")),
		defaultMerchantID: strings.TrimSpace(os.Getenv("CLICK_MERCHANT_ID")),
		merchantCfg: structs.MerchantConfig{
			MerchantUserID: strings.TrimSpace(os.Getenv("CLICK_MERCHANT_USER_ID")),
			SecretKey:      strings.TrimSpace(os.Getenv("CLICK_SECRET_KEY")),
		},
	}
}

func (s *service) BuildPayURL(ctx context.Context, req structs.PayLinkParams) (string, error) {
	_ = ctx

	// defaults
	if strings.TrimSpace(req.ServiceID) == "" {
		req.ServiceID = s.defaultServiceID
	}
	if strings.TrimSpace(req.MerchantID) == "" {
		req.MerchantID = s.defaultMerchantID
	}
	if strings.TrimSpace(req.MerchantUserID) == "" {
		req.MerchantUserID = s.merchantCfg.MerchantUserID
	}

	if strings.TrimSpace(req.ServiceID) == "" {
		return "", errors.New("service_id is required")
	}
	if strings.TrimSpace(req.MerchantID) == "" {
		return "", errors.New("merchant_id is required")
	}
	if strings.TrimSpace(req.Amount) == "" {
		return "", errors.New("amount is required")
	}
	if strings.TrimSpace(req.TransactionParam) == "" {
		return "", errors.New("transaction_param is required (maps to merchant_trans_id)")
	}

	u, err := url.Parse("https://my.click.uz/services/pay")
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("service_id", req.ServiceID)
	q.Set("merchant_id", req.MerchantID)
	q.Set("amount", req.Amount)
	q.Set("transaction_param", req.TransactionParam)

	if strings.TrimSpace(req.MerchantUserID) != "" {
		q.Set("merchant_user_id", req.MerchantUserID)
	}
	if strings.TrimSpace(req.ReturnURL) != "" {
		q.Set("return_url", req.ReturnURL)
	}
	if strings.TrimSpace(req.CardType) != "" {
		q.Set("card_type", req.CardType)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

const merchantBaseURL = "https://api.click.uz/v2/merchant"

func (s *service) authHeader(now time.Time) (string, error) {
	if strings.TrimSpace(s.merchantCfg.MerchantUserID) == "" || strings.TrimSpace(s.merchantCfg.SecretKey) == "" {
		return "", errors.New("CLICK_MERCHANT_USER_ID / CLICK_SECRET_KEY env missing")
	}

	ts := now.Unix()
	raw := strconv.FormatInt(ts, 10) + s.merchantCfg.SecretKey
	sum := sha1.Sum([]byte(raw))
	digest := hex.EncodeToString(sum[:])

	return fmt.Sprintf("%s:%s:%d", s.merchantCfg.MerchantUserID, digest, ts), nil
}

func (s *service) doJSON(ctx context.Context, method, fullURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return err
	}

	auth, err := s.authHeader(time.Now())
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Auth", auth)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("click merchant api http=%d body=%s", resp.StatusCode, string(b))
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

func (s *service) PaymentStatus(ctx context.Context, serviceID int64, paymentID int64) (structs.PaymentStatusResponse, error) {
	var out structs.PaymentStatusResponse
	u := fmt.Sprintf("%s/payment/status/%d/%d", merchantBaseURL, serviceID, paymentID)
	err := s.doJSON(ctx, http.MethodGet, u, &out)
	return out, err
}

func (s *service) PaymentStatusByMTI(ctx context.Context, serviceID int64, merchantTransID string, paymentDate time.Time) (structs.StatusByMTIResponse, error) {
	var out structs.StatusByMTIResponse
	date := paymentDate.Format("2006-01-02") // YYYY-MM-DD
	u := fmt.Sprintf("%s/payment/status_by_mti/%d/%s/%s", merchantBaseURL, serviceID, url.PathEscape(merchantTransID), date)
	err := s.doJSON(ctx, http.MethodGet, u, &out)
	return out, err
}

func (s *service) PaymentReversal(ctx context.Context, serviceID int64, paymentID int64) (structs.ReversalResponse, error) {
	var out structs.ReversalResponse
	u := fmt.Sprintf("%s/payment/reversal/%d/%d", merchantBaseURL, serviceID, paymentID)
	err := s.doJSON(ctx, http.MethodDelete, u, &out)
	return out, err
}
