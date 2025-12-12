package order

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sushitana/internal/payment/click"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"

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
	}
	service struct {
		orderRepo orderrepo.Repo
		logger    logger.Logger
		clickSvc  click.Service
	}
)

func New(p Params) Service {
	return &service{
		orderRepo: p.OrderRepo,
		logger:    p.Logger,
		clickSvc:  p.ClickSvc,
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
	serviceId := os.Getenv("CLICK_SERVICE_ID")
	merchantId := os.Getenv("CLICK_MERCHANT_ID")

	clickReq := structs.CheckoutPrepareRequest{
		ServiceID:        serviceId,
		MerchatID:        merchantId,
		Amount:           cast.ToString(order.Order.TotalPrice),
		TransactionParam: cast.ToString(order.Order.OrderNumber),
		ReturnUrl:        "",
		Description:      fmt.Sprintf("Sushitana buyurtma #%d", order.Order.OrderNumber),
		TotalPrice:       order.Order.TotalPrice,
	}

	prepareResp, err := s.clickSvc.CheckoutPrepare(ctx, clickReq)
	if err != nil {
		s.logger.Error(ctx, "->clickSvc.CheckoutPrepare failed", zap.Error(err), zap.String("order_id", id))
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "UNPAID",
		})
		return "", fmt.Errorf("checkout prepare failed: %w", err)
	}

	reqID := prepareResp.RequestId
	txParam := order.Order.OrderNumber
	if err := s.orderRepo.UpdateClickInfo(ctx, id, reqID, cast.ToString(txParam)); err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdateClickInfo failed", zap.Error(err), zap.String("order_id", id))
	}

	var payURL string
	if reqID != "" {
		payURL = fmt.Sprintf("https://my.click.uz/services/pay/%s", reqID)
	} else if cast.ToString(txParam) != "" {
		payURL = fmt.Sprintf("https://my.click.uz/%s", cast.ToString(txParam))
	} else {
		s.logger.Error(ctx, "no pay url or identifiers in prepare response", zap.String("order_id", id), zap.Any("prepareResp", prepareResp))
		return "", fmt.Errorf("no payment url returned from click prepare")
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
