package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"sushitana/internal/iiko"
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

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		OrderRepo orderrepo.Repo
		ClickRepo clickrepo.Repo

		ClickSvc click.Service
		ShopSvc  shopapi.Service
		PaymeSvc payme.Service
		IikoSvc  iiko.Service

		Logger logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateOrder) (string, error)
		ConfirmByOperator(ctx context.Context, orderID string) error
		GetByTgId(ctx context.Context, tgId int64) (structs.GetListOrderByTgIDResponse, error)
		GetByID(ctx context.Context, id string) (structs.GetListPrimaryKeyResponse, error)
		GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error)
		Delete(ctx context.Context, order_id string) error
		UpdateStatus(ctx context.Context, req structs.UpdateStatus) error
		UpdatePaymentStatus(ctx context.Context, req structs.UpdateStatus) error
		HandleIikoDeliveryOrderUpdate(ctx context.Context, evt structs.IikoWebhookDeliveryOrderUpdate) error
	}

	service struct {
		orderRepo orderrepo.Repo
		clickRepo clickrepo.Repo
		logger    logger.Logger

		clickSvc click.Service
		paymeSvc payme.Service
		shopSvc  shopapi.Service
		iikoSvc  iiko.Service
	}
)

func New(p Params) Service {
	return &service{
		orderRepo: p.OrderRepo,
		clickRepo: p.ClickRepo,
		logger:    p.Logger,
		clickSvc:  p.ClickSvc,
		paymeSvc:  p.PaymeSvc,
		shopSvc:   p.ShopSvc,
		iikoSvc:   p.IikoSvc,
	}
}

func NormalizeDeliveryType(v string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(v))
	switch s {
	case "DELIVERY":
		return "DELIVERY", nil
	case "PICKUP":
		return "PICKUP", nil
	}

	sl := strings.TrimSpace(strings.ToLower(v))
	switch sl {
	case "delivery":
		return "DELIVERY", nil
	case "pickup":
		return "PICKUP", nil
	}

	return "", structs.ErrBadRequest
}

func NormalizePaymentMethod(v string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(v))
	switch s {
	case "CASH", "CLICK", "PAYME":
		return s, nil
	}

	sl := strings.TrimSpace(strings.ToLower(v))
	switch sl {
	case "cash":
		return "CASH", nil
	case "click":
		return "CLICK", nil
	case "payme":
		return "PAYME", nil
	}

	return "", structs.ErrBadRequest
}

func (s *service) Create(ctx context.Context, req structs.CreateOrder) (string, error) {
	dt, err := NormalizeDeliveryType(req.DeliveryType)
	if err != nil {
		return "", err
	}
	pm, err := NormalizePaymentMethod(req.PaymentMethod)
	if err != nil {
		return "", err
	}
	req.DeliveryType = dt
	req.PaymentMethod = pm
	if req.DeliveryType == "PICKUP" && req.Address == nil {
		req.Address = &structs.Address{Lat: 0, Lng: 0, Name: "", DistanceKm: 0}
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

	ord, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByID after Create", zap.Error(err))
		return "", err
	}

	var payURL string
	switch req.PaymentMethod {
	case "CLICK":
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.Order.ID, Status: "WAITING_PAYMENT"})

		serviceId := os.Getenv("CLICK_SERVICE_ID")
		merchantId := os.Getenv("CLICK_MERCHANT_ID")
		if serviceId == "" || merchantId == "" {
			return "", fmt.Errorf("CLICK_SERVICE_ID yoki CLICK_MERCHANT_ID env not found")
		}

		merchantTransID := cast.ToString(ord.Order.OrderNumber)

		amountInt := ord.Order.TotalPrice
		amount := float64(amountInt)

		_, err := s.clickSvc.CheckoutPrepare(ctx, structs.CheckoutPrepareRequest{
			ServiceID:        serviceId,
			MerchantID:       merchantId,
			TransactionParam: merchantTransID,
			Amount:           amount,
			Description:      fmt.Sprintf("Order #%s", merchantTransID),
		})
		if err != nil {
			_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.Order.ID, Status: "WAITING_OPERATOR"})
			return "", fmt.Errorf("click checkout/prepare failed: %w", err)
		}

		inv := structs.Invoice{
			MerchantTransID: merchantTransID,
			OrderID:         sql.NullString{String: ord.Order.ID, Valid: true},
			TgID:            sql.NullInt64{Int64: ord.Order.TgID, Valid: true},
			CustomerPhone:   sql.NullString{String: ord.Phone, Valid: true},
			Amount:          cast.ToString(amountInt),
			Currency:        "UZS",
			Status:          "WAITING_PAYMENT",
		}
		if _, err := s.clickRepo.Create(ctx, inv); err != nil {
			return "", err
		}

		payURL = BuildClickPayURL(cast.ToInt64(serviceId), merchantId, int64(amountInt), merchantTransID, "")
		_ = s.orderRepo.AddLink(ctx, payURL, ord.Order.ID)
		return payURL, nil

	case "PAYME":
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.Order.ID, Status: "WAITING_PAYMENT"})

		merchantID := os.Getenv("PAYME_KASSA_ID")
		if merchantID == "" {
			return "", fmt.Errorf("PAYME_KASSA_ID env not found")
		}

		amountTiyin := int64(1000 * 100)

		payURL, err = s.paymeSvc.BuildPaymeCheckoutURL(merchantID, ord.Order.ID, amountTiyin)
		if err != nil {
			s.logger.Error(ctx, "->paymeSvc.BuildPaymeCheckoutURL failed", zap.Error(err), zap.String("order_id", id))
			_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.Order.ID, Status: "WAITING_OPERATOR"})
			return "", fmt.Errorf("build payme checkout url failed: %w", err)
		}
		return payURL, nil

	default:
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.Order.ID, Status: "WAITING_OPERATOR"})
		return "", nil
	}
}

func (s *service) ConfirmByOperator(ctx context.Context, orderID string) error {
	return s.sendToIikoIfAllowed(ctx, orderID)
}

func (s *service) sendToIikoIfAllowed(ctx context.Context, orderID string) error {
	ord, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByID in sendToIikoIfAllowed", zap.Error(err))
		return err
	}

	curStatus := strings.ToUpper(cast.ToString(ord.Order.Status))
	paymentMethod := strings.ToUpper(cast.ToString(ord.Order.PaymentMethod))
	paymentStatus := strings.ToUpper(cast.ToString(ord.Order.PaymentStatus))

	if curStatus != "COOKING" {
		return fmt.Errorf("order is not ready for operator confirm (status=%s)", curStatus)
	}
	if paymentMethod == "CLICK" || paymentMethod == "PAYME" {
		if paymentStatus != "PAID" {
			return fmt.Errorf("online order: payment not completed yet (paymentStatus=%s)", paymentStatus)
		}
	}

	iikoReq, err := buildCreateOrderForIiko(ord)
	if err != nil {
		return err
	}

	_, err = s.iikoSvc.CreateOrder(ctx, iikoReq)
	if err != nil {
		s.logger.Error(ctx, "->iikoSvc.CreateOrder failed", zap.Error(err), zap.String("order_id", orderID))
		return err
	}

	if err := s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
		OrderId: orderID,
		Status:  "SENT_TO_IIKO",
	}); err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdateStatus SENT_TO_IIKO failed", zap.Error(err))
		return err
	}

	return nil
}

func (s *service) UpdatePaymentStatus(ctx context.Context, req structs.UpdateStatus) error {
	pStatus := strings.ToUpper(strings.TrimSpace(cast.ToString(req.Status)))

	if err := s.orderRepo.UpdatePaymentStatus(ctx, structs.UpdateStatus{
		OrderId: req.OrderId,
		Status:  pStatus,
	}); err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdatePaymentStatus", zap.Error(err))
		return err
	}

	switch pStatus {
	case "PAID":
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: req.OrderId,
			Status:  "WAITING_OPERATOR",
		})
	case "PENDING", "UNPAID":
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: req.OrderId,
			Status:  "WAITING_PAYMENT",
		})
	}

	return nil
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
	if err := s.orderRepo.Delete(ctx, order_id); err != nil {
		s.logger.Error(ctx, "->orderRepo.Delete", zap.Error(err))
		return err
	}
	return nil
}

func (s *service) UpdateStatus(ctx context.Context, req structs.UpdateStatus) error {
	st := strings.ToUpper(strings.TrimSpace(req.Status))
	req.Status = st

	if st == "SENT_TO_IIKO" {
		return s.sendToIikoIfAllowed(ctx, req.OrderId)
	}

	if err := s.orderRepo.UpdateStatus(ctx, req); err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdateStatus", zap.Error(err))
		return err
	}
	return nil
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

func buildCreateOrderForIiko(ord structs.GetListPrimaryKeyResponse) (structs.CreateOrder, error) {
	var req structs.CreateOrder
	req.DeliveryType = cast.ToString(ord.Order.DeliveryType)
	req.PaymentMethod = cast.ToString(ord.Order.PaymentMethod)
	req.Address = &ord.Order.Address
	req.Products = ord.Order.Products
	return req, nil
}

func (s *service) HandleIikoDeliveryOrderUpdate(ctx context.Context, evt structs.IikoWebhookDeliveryOrderUpdate) error {
	// iiko webhook payload misoli: eventType, eventTime, organizationId, correlationId, eventInfo{id,posId,externalNumber,timestamp,creationStatus,errorInfo,order{...}} :contentReference[oaicite:5]{index=5}
	if strings.ToUpper(evt.EventType) != "DELIVERYORDERUPDATE" {
		return nil
	}

	orderID := strings.TrimSpace(evt.EventInfo.ID)
	if orderID == "" {
		return fmt.Errorf("iiko webhook: empty eventInfo.id")
	}

	if strings.ToUpper(strings.TrimSpace(evt.EventInfo.CreationStatus)) != "SUCCESS" {
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: orderID,
			Status:  "REJECTED",
		})
		return nil
	}

	iikoStatus := extractIikoOrderStatus(evt.EventInfo.Order)
	fmt.Println("extracy iiko order status", iikoStatus)
	newStatus := mapIikoStatusToOurStatus(iikoStatus)
	fmt.Println("map iiko status to our status", newStatus)
	if newStatus == "" {
		return nil
	}

	return s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
		OrderId: orderID,
		Status:  newStatus,
	})
}

func extractIikoOrderStatus(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	// ko'p uchraydigan nomlar
	if v, ok := m["status"].(string); ok {
		return v
	}
	if v, ok := m["deliveryStatus"].(string); ok {
		return v
	}
	if v, ok := m["state"].(string); ok {
		return v
	}
	return ""
}

func mapIikoStatusToOurStatus(iikoStatus string) string {
	s := strings.ToUpper(strings.TrimSpace(iikoStatus))
	switch {
	case strings.Contains(s, "CANCEL"):
		return "CANCELLED"
	case strings.Contains(s, "REJECT"):
		return "REJECTED"
	case strings.Contains(s, "COOK"):
		return "COOKING"
	case strings.Contains(s, "READY"):
		return "READY_FOR_PICKUP"
	case strings.Contains(s, "WAY") || strings.Contains(s, "COURIER") || strings.Contains(s, "DELIVERY"):
		return "ON_THE_WAY"
	case strings.Contains(s, "DELIVERED"):
		return "DELIVERED"
	case strings.Contains(s, "CLOSE") || strings.Contains(s, "COMPLETE"):
		return "COMPLETED"
	default:
		return ""
	}
}
