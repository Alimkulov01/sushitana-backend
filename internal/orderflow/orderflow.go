package orderflow

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sushitana/internal/iiko"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	rtws "sushitana/internal/ws"
	"sushitana/pkg/logger"
	clientrepo "sushitana/pkg/repository/postgres/client_repo"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	"sushitana/pkg/utils"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

type Service interface {
	SendToIikoIfAllowed(ctx context.Context, orderID string) error
	NotifyOrderStatusIfNeeded(ctx context.Context, orderID string, newStatus string)
}

type Params struct {
	fx.In
	Logger     logger.Logger
	OrderRepo  orderrepo.Repo
	ClientRepo clientrepo.Repo
	Bot        *tgbotapi.BotAPI `optional:"true"`
	Hub        *rtws.Hub        `optional:"true"`
	IikoSvc    iiko.Service
}

type service struct {
	logger     logger.Logger
	orderRepo  orderrepo.Repo
	clientRepo clientrepo.Repo
	iikoSvc    iiko.Service
	bot        *tgbotapi.BotAPI `optional:"true"`
	hub        *rtws.Hub        `optional:"true"`
}

func New(p Params) Service {
	return &service{
		logger:     p.Logger,
		orderRepo:  p.OrderRepo,
		clientRepo: p.ClientRepo,
		iikoSvc:    p.IikoSvc,
		hub:        p.Hub,
		bot:        p.Bot,
	}
}

func (s *service) SendToIikoIfAllowed(ctx context.Context, orderID string) error {
	ord, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}

	// 1) allaqachon yuborilgan bo'lsa
	if strings.TrimSpace(ord.Order.IIKOOrderID) != "" || strings.TrimSpace(ord.Order.IIKODeliveryID) != "" {
		s.logger.Info(ctx, "iiko: already sent, skip",
			zap.String("order_id", orderID),
			zap.String("iiko_order_id", ord.Order.IIKOOrderID),
			zap.String("iiko_delivery_id", ord.Order.IIKODeliveryID),
		)
		return nil
	}

	deliveryType := strings.ToUpper(strings.TrimSpace(ord.Order.DeliveryType))
	paymentMethod := strings.ToUpper(strings.TrimSpace(ord.Order.PaymentMethod))
	paymentStatus := strings.ToUpper(strings.TrimSpace(ord.Order.PaymentStatus))

	// 2) to'lov sharti:
	// CASH bo'lsa ruxsat
	// CLICK/PAYME bo'lsa faqat PAID bo'lsa ruxsat
	if paymentMethod == "CLICK" || paymentMethod == "PAYME" {
		if paymentStatus != "PAID" {
			s.logger.Info(ctx, "iiko: online payment not PAID, skip",
				zap.String("order_id", orderID),
				zap.String("payment_method", paymentMethod),
				zap.String("payment_status", paymentStatus),
			)
			return nil
		}
	}

	// 3) build request
	iikoReq, err := buildCreateOrderForIiko(ord)
	if err != nil {
		s.logger.Error(ctx, "buildCreateOrderForIiko failed",
			zap.String("order_id", orderID),
			zap.String("delivery_type", deliveryType),
			zap.Error(err),
		)
		return err
	}

	// 4) DELIVERY/PICKUP -> hozir ikkalasi ham deliveries/create orqali ketadi
	var resp structs.IikoCreateDeliveryResponse
	resp, err = s.iikoSvc.CreateOrder(ctx, iikoReq)
	if err != nil {
		s.logger.Error(ctx, "iiko create failed",
			zap.String("order_id", orderID),
			zap.String("delivery_type", deliveryType),
			zap.Error(err),
		)
		return err
	}

	// 5) iiko meta update
	_ = s.orderRepo.UpdateIikoMeta(ctx, orderID, resp.OrderInfo.ID, resp.OrderInfo.PosID, resp.CorrelationId)

	s.logger.Info(ctx, "iiko create success",
		zap.String("order_id", orderID),
		zap.String("delivery_type", deliveryType),
		zap.String("iiko_order_id", resp.OrderInfo.ID),
		zap.String("pos_id", resp.OrderInfo.PosID),
		zap.String("creation_status", resp.OrderInfo.CreationStatus),
		zap.String("correlation_id", resp.CorrelationId),
	)

	return nil
}

func buildCreateOrderForIiko(ord structs.GetListPrimaryKeyResponse) (structs.IikoCreateDeliveryRequest, error) {
	organizationID := strings.TrimSpace(os.Getenv("IIKO_ORGANIZATION_ID"))
	terminalGroupID := strings.TrimSpace(os.Getenv("IIKO_TERMINAL_GROUP_ID"))
	deliveryOrderTypeID := strings.TrimSpace(os.Getenv("IIKO_DELIVERY_ORDER_TYPE_ID"))
	pickupOrderTypeID := strings.TrimSpace(os.Getenv("IIKO_PICKUP_ORDER_TYPE_ID"))

	paymentCashID := strings.TrimSpace(os.Getenv("IIKO_PAYMENT_CASH_ID"))
	paymentClickID := strings.TrimSpace(os.Getenv("IIKO_PAYMENT_CLICK_ID"))
	paymentPaymeID := strings.TrimSpace(os.Getenv("IIKO_PAYMENT_PAYME_ID"))

	if organizationID == "" || terminalGroupID == "" {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("IIKO_ORGANIZATION_ID/IIKO_TERMINAL_GROUP_ID empty")
	}

	deliveryType := strings.ToUpper(strings.TrimSpace(ord.Order.DeliveryType))
	paymentMethod := strings.ToUpper(strings.TrimSpace(ord.Order.PaymentMethod))

	// 1) orderTypeId
	var orderTypeID string
	switch deliveryType {
	case "DELIVERY":
		orderTypeID = deliveryOrderTypeID
	case "PICKUP":
		orderTypeID = pickupOrderTypeID
	default:
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("unknown DeliveryType=%s", ord.Order.DeliveryType)
	}
	if strings.TrimSpace(orderTypeID) == "" {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("orderTypeId empty for DeliveryType=%s", ord.Order.DeliveryType)
	}

	// 2) payment mapping
	var (
		paymentTypeID       string
		paymentKind         string
		processedExternally bool
	)

	switch paymentMethod {
	case "CASH":
		paymentKind = "Cash"
		paymentTypeID = paymentCashID
	case "CLICK":
		paymentKind = "External"
		processedExternally = true
		paymentTypeID = paymentClickID
	case "PAYME":
		paymentKind = "External"
		processedExternally = true
		paymentTypeID = paymentPaymeID
	default:
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("unknown PaymentMethod=%s", ord.Order.PaymentMethod)
	}
	if strings.TrimSpace(paymentTypeID) == "" {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("paymentTypeId empty for PaymentMethod=%s", ord.Order.PaymentMethod)
	}

	// 3) items: products + box aggregation
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
	if err := addDeliveryFeeItem(&items, ord.Order.DeliveryType, ord.Order.DeliveryPrice); err != nil {
		return structs.IikoCreateDeliveryRequest{}, err
	}

	// 4) payment sum (orderPriceForIIKO)
	sum := float64(ord.Order.OrderPriceForIIKO)
	if sum <= 0 {
		return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("orderPriceForIIKO is 0; cannot build iiko payment sum")
	}

	// 5) phone
	phone := strings.TrimSpace(ord.Phone)
	if phone == "" {
		phone = strings.TrimSpace(ord.Order.Phone)
	}
	if phone == "" {
		phone = "+998000000000"
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

	// 6) DELIVERY requires deliveryPoint with coordinates
	if deliveryType == "DELIVERY" {
		a := ord.Order.Address
		if a == nil {
			return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("iiko delivery requires address (nil)")
		}

		lat := a.Lat
		lng := a.Lng

		if lat == 0 || lng == 0 {
			return structs.IikoCreateDeliveryRequest{}, fmt.Errorf("iiko delivery requires coordinates (lat/lng), got lat=%v lng=%v", lat, lng)
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
				Latitude:  lat,
				Longitude: lng,
			},
			Address: &structs.IikoAddress{
				Street:  street,
				House:   house,
				Comment: strings.TrimSpace(a.Name),
			},
		}
	}

	// 7) final request
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

func addDeliveryFeeItem(items *[]structs.IikoOrderItem, deliveryType string, deliveryPrice int64) error {
	dt := strings.ToUpper(strings.TrimSpace(deliveryType))
	if dt != "DELIVERY" {
		return nil
	}
	if deliveryPrice <= 0 {
		return nil
	}

	var envKey string
	switch deliveryPrice {
	case 7000:
		envKey = ""
	case 25000:
		envKey = "IIKO_DELIVERY_PRODUCT_ID_25000"
	default:
		return fmt.Errorf("unsupported deliveryPrice=%d (only 7000 or 25000)", deliveryPrice)
	}

	productID := strings.TrimSpace(os.Getenv(envKey))
	if productID == "" {
		return fmt.Errorf("%s env not set", envKey)
	}

	// duplicate bo'lib ketmasin
	for i := range *items {
		if strings.TrimSpace((*items)[i].ProductId) == productID {
			(*items)[i].Amount = 1
			(*items)[i].Type = "Product"
			return nil
		}
	}

	*items = append(*items, structs.IikoOrderItem{
		Type:      "Product",
		ProductId: productID,
		Amount:    1,
	})
	return nil
}

func (s *service) NotifyOrderStatusIfNeeded(ctx context.Context, orderID string, newStatus string) {
	if s.bot == nil {
		return
	}

	s.logger.Info(ctx, "NotifyOrderStatusIfNeeded called",
		zap.String("orderId", orderID),
		zap.String("newStatus", newStatus),
		zap.Bool("bot_nil", s.bot == nil),
	)

	target, ok, err := s.orderRepo.TryMarkNotified(ctx, orderID, newStatus)
	s.logger.Info(ctx, "TryMarkNotified result",
		zap.String("orderId", orderID),
		zap.String("newStatus", newStatus),
		zap.Bool("ok", ok),
		zap.Int64("tgId", target.TgID),
		zap.Int64("orderNumber", int64(target.OrderNumber)),
		zap.Error(err),
	)
	if err != nil {
		s.logger.Error(ctx, "TryMarkNotified failed",
			zap.String("orderId", orderID),
			zap.String("status", newStatus),
			zap.Error(err),
		)
		return
	}
	if !ok || target.TgID == 0 {
		return
	}
	if s.hub != nil {
		s.hub.BroadcastToUser(target.TgID, structs.Event{
			Type: structs.EventOrderPatch,
			Payload: structs.OrderPatchPayload{
				ID:            orderID,
				PaymentStatus: newStatus,
				OrderNumber:   int64(target.OrderNumber),
			},
		})
	}
	if newStatus == "WAITING_OPERATOR" {
		return
	}

	lang := utils.UZ // default
	if s.clientRepo != nil {
		l, e := s.clientRepo.GetLanguageByTgID(ctx, target.TgID)
		if e == nil {
			if ll, ok := toLang(l); ok {
				lang = ll
			}
		}
	}

	key := statusTextKey(newStatus)
	statusText := newStatus
	if key != "" {
		statusText = texts.Get(lang, key)
	}

	msg := fmt.Sprintf("📦 Zakaz #%d holati: %s", target.OrderNumber, statusText)
	_, e := s.bot.Send(tgbotapi.NewMessage(target.TgID, msg))
	if e != nil {
		s.logger.Warn(ctx, "Telegram notify failed",
			zap.Int64("tg_id", target.TgID),
			zap.Error(e),
		)
	}
}

func toLang(s string) (utils.Lang, bool) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "uz":
		return utils.UZ, true
	case "ru":
		return utils.RU, true
	case "en":
		return utils.EN, true
	default:
		return utils.UZ, false
	}
}

func statusTextKey(st string) texts.TextKey {
	switch st {
	case "WAITING_PAYMENT":
		return texts.OrderStatusWaitingPayment
	case "WAITING_OPERATOR":
		return texts.OrderStatusWaitingOperator
	case "COOKING":
		return texts.OrderStatusCooking
	case "READY_FOR_PICKUP":
		return texts.OrderStatusReadyForPickup
	case "ON_THE_WAY":
		return texts.OrderStatusOnTheWay
	case "DELIVERED":
		return texts.OrderStatusDelivered
	case "COMPLETED":
		return texts.OrderStatusCompleted
	case "CANCELLED":
		return texts.OrderStatusCancelled
	case "REJECTED":
		return texts.OrderStatusRejected
	default:
		return ""
	}
}
