package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sushitana/internal/payment/click"
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
	}
)

func New(p Params) Service {
	return &service{
		orderRepo: p.OrderRepo,
		logger:    p.Logger,
		clickSvc:  p.ClickSvc,
		clickRepo: p.ClickRepo,
	}
}
func (s *service) Create(ctx context.Context, req structs.CreateOrder) (string, error) {
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
	case "CLICK", "PAYME":
		serviceId := os.Getenv("CLICK_SERVICE_ID")
		merchantId := os.Getenv("CLICK_MERCHANT_ID")

		merchantTransID := order.Order.OrderNumber

		clickReq := structs.CheckoutPrepareRequest{
			ServiceID:        serviceId,
			MerchantID:       merchantId,
			TransactionParam: cast.ToString(merchantTransID), // MUHIM: callback bilan bir xil bo'lishi kerak
			Amount:           1000.0,                         // real amount qo'ying
			ReturnUrl:        os.Getenv("CLICK_RETURN_URL"),
			Description:      fmt.Sprintf("Sushitana buyurtma #%d", order.Order.OrderNumber),
			Items:            nil,
		}

		prepareResp, err := s.clickSvc.CheckoutPrepare(ctx, clickReq)
		if err != nil {
			s.logger.Error(ctx, "->clickSvc.CheckoutPrepare failed", zap.Error(err), zap.String("order_id", id))
			_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: order.Order.ID, Status: "UNPAID"})
			return "", fmt.Errorf("checkout prepare failed: %w", err)
		}

		invoiceResp, err := s.clickSvc.CheckoutInvoice(ctx, structs.CheckoutInvoiceRequest{
			RequestId:   prepareResp.RequestId,
			PhoneNumber: order.Phone,
		})
		if err != nil {
			s.logger.Error(ctx, "->clickSvc.CheckoutInvoice failed", zap.Error(err), zap.String("order_id", id))
			_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: order.Order.ID, Status: "UNPAID"})
			return "", fmt.Errorf("checkout invoice failed: %w", err)
		}

		// 1) invoices jadvaliga yozamiz (callback Prepare kelishidan oldin!)
		inv := structs.Invoice{
			ClickInvoiceID:  invoiceResp.InvoiceId, // sizning struct field nomingizga moslang
			MerchantTransID: cast.ToString(merchantTransID),
			OrderID:         sql.NullString{String: order.Order.ID, Valid: true},
			TgID:            sql.NullInt64{Int64: order.Order.TgID, Valid: true},
			CustomerPhone:   sql.NullString{String: order.Phone, Valid: true},
			Amount:          cast.ToString(order.Order.TotalCount), // yoki string/decimal bo'lsa moslang
			Currency:        "UZS",
			Status:          "WAITING_PAYMENT",
			Comment:         sql.NullString{String: prepareResp.RequestId, Valid: true}, // ixtiyoriy: request_id ni commentga saqlab qo'yish
		}

		if err := s.clickRepo.Create(ctx, inv); err != nil {
			s.logger.Error(ctx, "->clickRepo.Create invoice failed", zap.Error(err), zap.String("order_id", id))
			// xohlasangiz order status UNPAID qilib qo'ying va return qiling
		}

		// 2) pay URL
		reqID := prepareResp.RequestId
		if reqID == "" {
			return "", fmt.Errorf("no request_id returned from click prepare")
		}
		payURL = fmt.Sprintf("https://my.click.uz/services/pay/%s", reqID)

		// 3) order status
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "WAITING_PAYMENT",
		})

	default:
		if err := s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "WAITING_OPERATOR",
		}); err != nil {
			s.logger.Error(ctx, "->orderRepo.UpdateStatus (CREATED) failed", zap.Error(err), zap.String("order_id", id))
		}
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
