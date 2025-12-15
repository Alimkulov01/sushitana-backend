package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sushitana/internal/payment/click"
	"sushitana/internal/payment/payme"
	shopapi "sushitana/internal/payment/shop-api"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	clickrepo "sushitana/pkg/repository/postgres/payment_repo/click_repo"

	"github.com/spf13/cast"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type DeliveryMethod string

const (
	DeliveryMethodDelivery DeliveryMethod = "DELIVERY"
	DeliveryMethodPickup   DeliveryMethod = "PICKUP"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		OrderRepo orderrepo.Repo
		ClickRepo clickrepo.Repo
		ClickSvc  click.Service
		ShopSvc   shopapi.Service
		PaymeSvc  payme.Service
		Logger    logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateOrder) (string, error)
		GetByTgId(ctx context.Context, tgId int64) (structs.GetListOrderByTgIDResponse, error)
		GetByID(ctx context.Context, id string) (structs.GetListPrimaryKeyResponse, error)
		GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error)
		Delete(ctx context.Context, order_id string) error
		UpdateStatus(ctx context.Context, req structs.UpdateStatus) error
		UpdatePaymentStatus(ctx context.Context, req structs.UpdateStatus) error
	}
	service struct {
		orderRepo orderrepo.Repo
		clickRepo clickrepo.Repo
		logger    logger.Logger
		clickSvc  click.Service
		paymeSvc  payme.Service
		shopSvc   shopapi.Service
	}
)

func New(p Params) Service {
	return &service{
		orderRepo: p.OrderRepo,
		logger:    p.Logger,
		clickSvc:  p.ClickSvc,
		clickRepo: p.ClickRepo,
		paymeSvc:  p.PaymeSvc,
		shopSvc:   p.ShopSvc,
	}
}
func (s *service) Create(ctx context.Context, req structs.CreateOrder) (string, error) {
	if req.DeliveryType == "PICKUP" && req.Address == nil {
		req.Address = &structs.Address{
			Lat:        0,
			Lng:        0,
			Name:       "",
			DistanceKm: 0,
		}
	}

	if req.DeliveryType == "DELIVERY" && req.Address == nil {
		return "", fmt.Errorf("address is required for delivery")
	}
	id, err := s.orderRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return "", err
		}
		s.logger.Error(ctx, "->orderRepo.Create", zap.Error(err))
		return "", err
	}

	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByID after Create", zap.Error(err))
		return "", err
	}

	var payURL string

	switch req.PaymentMethod {
	case "CLICK":
		serviceId := os.Getenv("CLICK_SERVICE_ID")
		merchantId := os.Getenv("CLICK_MERCHANT_ID")
		if serviceId == "" || merchantId == "" {
			return "", fmt.Errorf("CLICK_SERVICE_ID yoki CLICK_MERCHANT_ID env yo‘q")
		}

		merchantTransID := cast.ToString(order.Order.OrderNumber)
		amountInt := 1000
		amount := float64(amountInt)

		_, err := s.clickSvc.CheckoutPrepare(ctx, structs.CheckoutPrepareRequest{
			ServiceID:        serviceId,
			MerchantID:       merchantId,
			TransactionParam: merchantTransID,
			Amount:           amount,
			ReturnUrl:        "order",
			Description:      fmt.Sprintf("Order #%s", merchantTransID),
		})
		if err != nil {
			_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: order.Order.ID, Status: "UNPAID"})
			return "", fmt.Errorf("click checkout/prepare failed: %w", err)
		}

		inv := structs.Invoice{
			// ClickInvoiceID:  invResp.InvoiceId,
			MerchantTransID: merchantTransID,
			OrderID:         sql.NullString{String: order.Order.ID, Valid: true},
			TgID:            sql.NullInt64{Int64: order.Order.TgID, Valid: true},
			CustomerPhone:   sql.NullString{String: order.Phone, Valid: true},
			Amount:          cast.ToString(amountInt),
			Currency:        "UZS",
			Status:          "WAITING_PAYMENT",
		}
		_, err = s.clickRepo.Create(ctx, inv)
		if err != nil {
			return "", err
		}

		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "WAITING_PAYMENT",
		})

		payURL = BuildClickPayURL(cast.ToInt64(serviceId), merchantId, int64(amountInt), merchantTransID, "order")
		return payURL, nil

	case "PAYME":
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "WAITING_PAYMENT",
		})
		merchantID := os.Getenv("PAYME_KASSA_ID")
		amountTiyin := int64(1000 * 100)        
		payURL, err = s.paymeSvc.BuildPaymeCheckoutURL(merchantID, order.Order.ID, amountTiyin)
		if err != nil {
			s.logger.Error(ctx, "->paymeSvc.BuildPaymeCheckoutURL failed", zap.Error(err), zap.String("order_id", id))
			_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: order.Order.ID, Status: "UNPAID"})
			return "", fmt.Errorf("build payme checkout url failed: %w", err)
		}

	default:
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "WAITING_OPERATOR",
		})
	}

	return payURL, nil
}

func (s *service) GetByTgId(ctx context.Context, tgId int64) (structs.GetListOrderByTgIDResponse, error) {
	resp, err := s.orderRepo.GetByTgId(ctx, tgId)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByTgId", zap.Error(err))
		return structs.GetListOrderByTgIDResponse{}, err
	}
	return resp, nil
}

func (s *service) GetByID(ctx context.Context, id string) (structs.GetListPrimaryKeyResponse, error) {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByID", zap.Error(err))
		return structs.GetListPrimaryKeyResponse{}, err
	}
	return order, nil
}

func (s *service) GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error) {
	resp, err := s.orderRepo.GetList(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetList", zap.Error(err))
		return structs.GetListOrderResponse{}, err
	}
	return resp, nil
}

func (s *service) Delete(ctx context.Context, order_id string) error {
	err := s.orderRepo.Delete(ctx, order_id)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.Delete", zap.Error(err))
		return err
	}
	return nil
}

func (s *service) UpdateStatus(ctx context.Context, req structs.UpdateStatus) error {
	err := s.orderRepo.UpdateStatus(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdateStatus", zap.Error(err))
		return err
	}
	return nil
}
func (s *service) UpdatePaymentStatus(ctx context.Context, req structs.UpdateStatus) error {
	err := s.orderRepo.UpdatePaymentStatus(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdatePaymentStatus", zap.Error(err))
		return err
	}
	return nil
}

func ParseDeliveryMethod(v string) (DeliveryMethod, error) {
	switch v {
	case "delivery":
		return DeliveryMethodDelivery, nil
	case "pickup":
		return DeliveryMethodPickup, nil
	default:
		return "", structs.ErrBadRequest
	}
}

func BuildClickPayURL(serviceID int64, merchantID string, amountInt int64, orderID, returnURL string) string {
	v := url.Values{}
	v.Set("service_id", cast.ToString(serviceID))
	v.Set("merchant_id", merchantID)
	v.Set("amount", fmt.Sprintf("%d.00", amountInt))
	v.Set("transaction_param", orderID)             
	if returnURL != "" {
		v.Set("return_url", returnURL)
	}
	return "https://my.click.uz/services/pay?" + v.Encode()
}
