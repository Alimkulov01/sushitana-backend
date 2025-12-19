package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
		HandleIikoDeliveryOrderUpdate(ctx context.Context, evt structs.IikoWebhookEvent) error
		HandleIikoDeliveryOrderError(ctx context.Context, evt structs.IikoWebhookEvent) error
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

		amountTiyin := ord.Order.TotalPrice * 100

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

	// ✅ 0) Agar allaqachon iiko'ga yuborilgan bo‘lsa — qaytamiz (double send bo‘lmasin)
	// Sizda qaysi fieldlar borligiga qarab moslang: iiko_order_id / iiko_delivery_id / status
	if strings.TrimSpace(cast.ToString(ord.Order.IIKOOrderID)) != "" ||
		strings.TrimSpace(cast.ToString(ord.Order.IIKODeliveryID)) != "" ||
		curStatus == "SENT_TO_IIKO" {
		return nil
	}

	if paymentMethod == "CLICK" || paymentMethod == "PAYME" {
		if paymentStatus != "PAID" {
			return fmt.Errorf("online order: payment not completed yet (paymentStatus=%s)", paymentStatus)
		}
	} else {
		// CASH bo‘lsa: operator tasdiqlamaguncha yubormaymiz (sizning flow)
		if curStatus != "WAITING_OPERATOR" {
			return fmt.Errorf("cash order is not ready for operator confirm (status=%s)", curStatus)
		}
	}

	iikoReq, err := buildCreateOrderForIiko(ord)
	if err != nil {
		return err
	}

	ctx2, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	resp, err := s.iikoSvc.CreateOrder(ctx2, iikoReq)
	if err != nil {
		s.logger.Error(ctx, "->iikoSvc.CreateOrder failed", zap.Error(err), zap.String("order_id", orderID))
		return err
	}

	if err := s.orderRepo.UpdateIikoMeta(ctx, orderID, resp.OrderInfo.ID, resp.OrderInfo.PosID, resp.CorrelationId); err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdateIikoMeta failed", zap.Error(err), zap.String("order_id", orderID))
		return err
	}

	// ✅ 3) status: xohlasangiz SENT_TO_IIKO qiling, yoki statusni o‘zgartirmasdan ham bo‘ladi
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
	pStatus := strings.ToUpper(strings.TrimSpace(req.Status))
	if err := s.orderRepo.UpdatePaymentStatus(ctx, structs.UpdateStatus{
		OrderId: req.OrderId,
		Status:  pStatus,
	}); err != nil {
		return err
	}

	ord, err := s.orderRepo.GetByID(ctx, req.OrderId)
	if err != nil {
		return err
	}

	paymentMethod := strings.ToUpper(ord.Order.PaymentMethod)
	deliveryType := strings.ToUpper(ord.Order.DeliveryType)

	switch pStatus {
	case "PAID":
		if paymentMethod == "CLICK" || paymentMethod == "PAYME" {

			// address bo'lmasa yubormaymiz
			if deliveryType == "DELIVERY" && ord.Order.Address == nil {
				return fmt.Errorf("paid but address missing")
			}

			// iiko create (async tavsiya)
			go func(orderID string) {
				_ = s.sendToIikoIfAllowed(context.Background(), orderID)
			}(req.OrderId)
			return nil
		}

		// CASH bo'lsa PAID bo'lmaydi odatda; lekin bo'lsa ham ixtiyoriy:
		return nil

	case "PENDING", "UNPAID":
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: req.OrderId, Status: "WAITING_PAYMENT"})
		return nil
	default:
		return nil
	}
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

	if st == "COOKING" {
		if err := s.sendToIikoIfAllowed(ctx, req.OrderId); err != nil {
			return err
		}
		return nil
	}

	return s.orderRepo.UpdateStatus(ctx, req)
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

func buildCreateOrderForIiko(ord structs.GetListPrimaryKeyResponse) (structs.IikoCreateDeliveryRequest, error) {
	organizationID := os.Getenv("IIKO_ORGANIZATION_ID")
	terminalGroupID := os.Getenv("IIKO_TERMINAL_GROUP_ID")
	deliveryOrderTypeID := os.Getenv("IIKO_DELIVERY_ORDER_TYPE_ID")
	pickupOrderTypeID := os.Getenv("IIKO_PICKUP_ORDER_TYPE_ID")

	paymentCashID := os.Getenv("IIKO_PAYMENT_CASH_ID")
	paymentClickID := os.Getenv("IIKO_PAYMENT_CLICK_ID")
	paymentPaymeID := os.Getenv("IIKO_PAYMENT_PAYME_ID")

	if organizationID == "" || terminalGroupID == "" {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("IIKO_ORGANIZATION_ID/IIKO_TERMINAL_GROUP_ID empty")
	}

	deliveryType := strings.ToUpper(strings.TrimSpace(ord.Order.DeliveryType))
	paymentMethod := strings.ToUpper(strings.TrimSpace(ord.Order.PaymentMethod))

	var orderTypeID string
	switch deliveryType {
	case "DELIVERY":
		orderTypeID = strings.TrimSpace(deliveryOrderTypeID)
	case "PICKUP":
		orderTypeID = strings.TrimSpace(pickupOrderTypeID)
	default:
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("unknown DeliveryType=%s", ord.Order.DeliveryType)
	}
	if orderTypeID == "" {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("orderTypeId empty for DeliveryType=%s", ord.Order.DeliveryType)
	}

	var (
		paymentTypeID       string
		paymentKind         string
		processedExternally bool
	)

	switch paymentMethod {
	case "CASH":
		paymentKind = "Cash"
		paymentTypeID = strings.TrimSpace(paymentCashID)
	case "CLICK":
		paymentKind = "Card"
		processedExternally = true
		paymentTypeID = strings.TrimSpace(paymentClickID)
	case "PAYME":
		paymentKind = "Card"
		processedExternally = true
		paymentTypeID = strings.TrimSpace(paymentPaymeID)
	default:
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("unknown PaymentMethod=%s", ord.Order.PaymentMethod)
	}
	if paymentTypeID == "" {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("paymentTypeId empty for PaymentMethod=%s", ord.Order.PaymentMethod)
	}

	boxQty := make(map[string]float64, len(ord.Order.Products))
	for _, p := range ord.Order.Products {
		boxID := strings.TrimSpace(p.BoxID)
		if boxID == "" {
			continue
		}
		boxQty[boxID] += float64(p.Quantity)
	}

	items := make([]structs.IikoOrderItem, 0, len(ord.Order.Products)+len(boxQty))

	for _, p := range ord.Order.Products {
		items = append(items, structs.IikoOrderItem{
			Type:      "Product",
			ProductId: p.ID,
			Amount:    float64(p.Quantity),
		})
	}

	for boxID, amt := range boxQty {
		items = append(items, structs.IikoOrderItem{
			Type:      "Product",
			ProductId: boxID,
			Amount:    amt,
		})
	}

	sum := float64(ord.Order.OrderPriceForIIKO)
	if sum <= 0 {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("orderPriceForIIKO is 0; cannot build iiko payment sum")
	}

	phone := strings.TrimSpace(ord.Phone)
	if phone == "" {
		phone = strings.TrimSpace(ord.Order.Phone)
	}

	comment := strings.TrimSpace(ord.Order.Comment)

	iikoOrder := structs.IikoOrder{
		Phone:          phone,
		ExternalNumber: fmt.Sprintf("%d", ord.Order.OrderNumber),
		OrderTypeId:    orderTypeID,
		Comment:        comment,
		Items:          items,
		Payments: []structs.IikoPayment{
			{
				PaymentTypeId:         paymentTypeID,
				PaymentTypeKind:       paymentKind,
				Sum:                   sum,
				IsProcessedExternally: processedExternally,
			},
		},
	}

	if deliveryType == "DELIVERY" {
		a := ord.Order.Address
		if a == nil {
			return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("delivery address is nil")
		}

		house := strings.TrimSpace(a.House)
		if house == "" {
			house = "1"
		}

		streetName := strings.TrimSpace(a.Street)
		if streetName == "" {
			streetName = strings.TrimSpace(a.Name)
		}

		var street *structs.IikoStreet
		if streetName != "" {
			street = &structs.IikoStreet{Name: streetName}
		}

		iikoOrder.DeliveryPoint = &structs.IikoDeliveryPoint{
			Coordinates: &structs.IikoCoordinates{
				Latitude:  a.Lat,
				Longitude: a.Lng,
			},
			Address: &structs.IikoAddress{
				Street:  street,
				House:   house,
				Comment: a.Name,
			},
		}
	}

	return structs.IikoCreateDeliveryRequest{
		OrganizationId:  organizationID,
		TerminalGroupId: terminalGroupID,
		CreateOrderSettings: &structs.IikoCreateSettings{
			TransportToFrontTimeout: 300,
			CheckStopList:           false,
		},
		Order: iikoOrder,
	}, nil
}
func (s *service) HandleIikoDeliveryOrderUpdate(ctx context.Context, evt structs.IikoWebhookEvent) error {
	s.logger.Info(ctx, "IIKO webhook handling",
		zap.String("eventType", evt.EventType),
		zap.String("externalNumber", evt.EventInfo.ExternalNumber),
		zap.String("creationStatus", evt.EventInfo.CreationStatus),
	)
	if strings.ToUpper(evt.EventType) != "DELIVERYORDERUPDATE" {
		return nil
	}

	ext := strings.TrimSpace(evt.EventInfo.ExternalNumber)
	if ext == "" {
		return fmt.Errorf("iiko webhook: empty externalNumber")
	}

	num, err := strconv.ParseInt(ext, 10, 64)
	if err != nil {
		s.logger.Warn(ctx, "IIKO webhook bad externalNumber",
			zap.String("externalNumber", ext),
			zap.Error(err),
		)
		return fmt.Errorf("iiko webhook: bad externalNumber=%q: %w", ext, err)
	}

	ord, err := s.orderRepo.GetByOrderNumber(ctx, num)
	if err != nil {
		s.logger.Error(ctx, "IIKO webhook GetByOrderNumber failed",
			zap.Int64("orderNumber", num),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info(ctx, "IIKO webhook matched local order",
		zap.String("orderId", ord.ID),
		zap.Int64("orderNumber", ord.OrderNumber),
		zap.String("currentStatus", ord.Status),
		zap.String("currentPaymentStatus", ord.PaymentStatus),
	)

	if strings.ToUpper(strings.TrimSpace(evt.EventInfo.CreationStatus)) != "SUCCESS" {
		s.logger.Warn(ctx, "IIKO webhook creationStatus not SUCCESS -> REJECTED",
			zap.String("creationStatus", evt.EventInfo.CreationStatus),
			zap.String("orderId", ord.ID),
		)
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.ID, Status: "REJECTED"})
		return nil
	}
	if err := s.orderRepo.UpdateIikoMeta(ctx, ord.ID, evt.EventInfo.ID, evt.EventInfo.PosID, evt.CorrelationId); err != nil {
		s.logger.Error(ctx, "IIKO webhook UpdateIikoMeta failed",
			zap.String("orderId", ord.ID),
			zap.Error(err),
		)
	} else {
		s.logger.Info(ctx, "IIKO webhook iiko meta updated",
			zap.String("orderId", ord.ID),
			zap.String("iikoOrderId", evt.EventInfo.ID),
			zap.String("posId", evt.EventInfo.PosID),
			zap.String("correlationId", evt.CorrelationId),
		)
	}
	iikoStatus := extractIikoOrderStatus(evt.EventInfo.Order)
	newStatus := mapIikoStatusToOurStatus(iikoStatus)

	s.logger.Info(ctx, "IIKO webhook status mapping",
		zap.String("orderId", ord.ID),
		zap.String("iikoStatus", iikoStatus),
		zap.String("mappedStatus", newStatus),
	)

	if newStatus == "" {
		return nil
	}

	if err := s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.ID, Status: newStatus}); err != nil {
		s.logger.Error(ctx, "IIKO webhook UpdateStatus failed",
			zap.String("orderId", ord.ID),
			zap.String("status", newStatus),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info(ctx, "IIKO webhook UpdateStatus OK",
		zap.String("orderId", ord.ID),
		zap.String("status", newStatus),
	)
	return nil
}

func (s *service) HandleIikoDeliveryOrderError(ctx context.Context, evt structs.IikoWebhookEvent) error {
	if strings.ToUpper(strings.TrimSpace(evt.EventType)) != "DELIVERYORDERERROR" {
		return nil
	}

	ext := strings.TrimSpace(evt.EventInfo.ExternalNumber)
	if ext == "" {
		return fmt.Errorf("iiko webhook error: empty externalNumber")
	}

	num, err := strconv.ParseInt(ext, 10, 64)
	if err != nil {
		return fmt.Errorf("iiko webhook error: bad externalNumber=%q: %w", ext, err)
	}

	ord, err := s.orderRepo.GetByOrderNumber(ctx, num)
	if err != nil {
		return err
	}

	_ = s.orderRepo.UpdateIikoMeta(ctx, ord.ID, evt.EventInfo.ID, evt.EventInfo.PosID, evt.CorrelationId)

	// bu yerda siz REJECTED yoki FAILED_IIKO kabi status ishlating
	_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.ID, Status: "REJECTED"})

	// xohlasangiz errorInfo ni log qiling
	if evt.EventInfo.ErrorInfo != nil {
		s.logger.Error(ctx, "IIKO order creation error",
			zap.String("order_id", ord.ID),
			zap.String("externalNumber", ext),
			zap.String("code", evt.EventInfo.ErrorInfo.Code),
			zap.String("message", evt.EventInfo.ErrorInfo.Message),
			zap.String("description", evt.EventInfo.ErrorInfo.Description),
		)
	}
	return nil
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
