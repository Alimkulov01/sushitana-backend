package order

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"sushitana/internal/iiko"
	"sushitana/internal/payment/click"
	"sushitana/internal/payment/payme"
	shopapi "sushitana/internal/payment/shop-api"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	rtws "sushitana/internal/ws"
	"sushitana/pkg/logger"
	"sushitana/pkg/utils"

	clientrepo "sushitana/pkg/repository/postgres/client_repo"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	clickrepo "sushitana/pkg/repository/postgres/payment_repo/click_repo"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"github.com/spf13/cast"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	ohangaronMin = int64(400000)
	olmaliqMin   = int64(60000)
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In

		OrderRepo  orderrepo.Repo
		ClickRepo  clickrepo.Repo
		ClientRepo clientrepo.Repo
		Bot        *tgbotapi.BotAPI `optional:"true"`
		Hub        *rtws.Hub        `optional:"true"`
		Zones      *utils.ZoneChecker

		ClickSvc click.Service
		ShopSvc  shopapi.Service
		PaymeSvc payme.Service
		IikoSvc  iiko.Service

		Logger logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateOrder) (string, string, error)
		ConfirmByOperator(ctx context.Context, orderID string) error
		GetByTgId(ctx context.Context, tgId int64) (structs.GetListOrderByTgIDResponse, error)
		GetByID(ctx context.Context, id string) (structs.GetListPrimaryKeyResponse, error)
		GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error)
		Delete(ctx context.Context, order_id string) error
		UpdateStatus(ctx context.Context, req structs.UpdateStatus) error
		UpdatePaymentStatus(ctx context.Context, req structs.UpdateStatus) error
		DeliveryMapFound(ctx context.Context, req structs.MapFoundRequest) (int64, bool, error)

		HandleIikoDeliveryOrderUpdate(ctx context.Context, evt structs.IikoWebhookEvent) error
		HandleIikoDeliveryOrderError(ctx context.Context, evt structs.IikoWebhookEvent) error

		BuildClickPayURL(serviceID int64, merchantID string, amountInt int64, orderID, returnURL string) string
	}

	service struct {
		orderRepo  orderrepo.Repo
		clickRepo  clickrepo.Repo
		clientRepo clientrepo.Repo
		bot        *tgbotapi.BotAPI `optional:"true"`
		hub        *rtws.Hub        `optional:"true"`
		zones      *utils.ZoneChecker

		logger logger.Logger

		clickSvc click.Service
		paymeSvc payme.Service
		shopSvc  shopapi.Service
		iikoSvc  iiko.Service
	}
)

func New(p Params) Service {
	return &service{
		orderRepo:  p.OrderRepo,
		clickRepo:  p.ClickRepo,
		clientRepo: p.ClientRepo,

		logger:   p.Logger,
		clickSvc: p.ClickSvc,
		paymeSvc: p.PaymeSvc,
		shopSvc:  p.ShopSvc,
		iikoSvc:  p.IikoSvc,
		zones:    p.Zones,
		hub:      p.Hub,
		bot:      p.Bot,
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

func (s *service) Create(ctx context.Context, req structs.CreateOrder) (string, string, error) {
	// 0) normalize
	dt, err := NormalizeDeliveryType(req.DeliveryType)
	if err != nil {
		return "", "", err
	}
	pm, err := NormalizePaymentMethod(req.PaymentMethod)
	if err != nil {
		return "", "", err
	}
	req.DeliveryType = dt
	req.PaymentMethod = pm

	// 1) validate products
	if len(req.Products) == 0 {
		return "", "", structs.ErrBadRequest // yaxshisi: ErrCartEmpty
	}
	for _, it := range req.Products {
		if strings.TrimSpace(it.ID) == "" || it.Quantity <= 0 {
			return "", "", structs.ErrBadRequest
		}
	}

	// 2) delivery type validate + zone check
	var zoneIdx int
	switch req.DeliveryType {
	case "PICKUP":
		if req.Address == nil {
			req.Address = &structs.Address{Lat: 0, Lng: 0, Name: "", DistanceKm: 0}
		}
		req.DeliveryPrice = 0

	case "DELIVERY":
		if req.Address == nil {
			return "", "", structs.ErrBadRequest
		}

		ok, idx, err := s.zones.ContainsAnyWithIndex(req.Address.Lat, req.Address.Lng)
		if err != nil {
			return "", "", fmt.Errorf("zone check failed: %w", err)
		}
		if !ok {
			return "", "", structs.ErrOutOfDeliveryZone
		}
		zoneIdx = idx

	default:
		return "", "", structs.ErrBadRequest
	}

	// 3) productsTotal'ni DB’dan hisoblaymiz (box ham qo‘shiladi)
	//    (min order DELIVERY uchun faqat mahsulotlar/box summasi, delivery kirmaydi)
	var (
		prodCache  = map[string]structs.ProductMeta{}
		boxCache   = map[string]structs.BoxMeta{}
		orderTotal int64
		boxTotal   int64
	)

	for i := range req.Products {
		pid := strings.TrimSpace(req.Products[i].ID)

		pm, ok := prodCache[pid]
		if !ok {
			price, name, url, boxID, err := s.orderRepo.GetProductPriceWithBox(ctx, pid)
			if err != nil {
				s.logger.Warn(ctx, "product price not found", zap.String("product_id", pid), zap.Error(err))
				return "", "", structs.ErrBadRequest
			}
			pm = structs.ProductMeta{
				Price: price,
				Name:  name,
				Url:   url,
				BoxID: strings.TrimSpace(boxID),
			}
			prodCache[pid] = pm
		}

		qty := req.Products[i].Quantity
		orderTotal += pm.Price * qty

		// req.Products ichini ham boyitib qo'yamiz (orders.items JSONB'ga shular tushadi)
		req.Products[i].ProductName = pm.Name
		req.Products[i].ProductPrice = pm.Price
		req.Products[i].ProductUrl = pm.Url
		req.Products[i].BoxID = pm.BoxID

		// box hisoblash
		if pm.BoxID != "" {
			bm, ok := boxCache[pm.BoxID]
			if !ok {
				bp, bn, _, _, err := s.orderRepo.GetProductPriceWithBox(ctx, pm.BoxID)
				if err != nil {
					s.logger.Warn(ctx, "box price not found", zap.String("box_id", pm.BoxID), zap.Error(err))
					// box yo'q bo'lsa ham orderni bloklamaslikni xohlasangiz: continue qiling
					return "", "", structs.ErrBadRequest
				}
				bm = structs.BoxMeta{Price: bp, Name: bn}
				boxCache[pm.BoxID] = bm
			}
			boxTotal += bm.Price * qty

			req.Products[i].BoxName = bm.Name
			req.Products[i].BoxPrice = bm.Price
		}
	}

	productsTotal := orderTotal + boxTotal

	// 4) delivery price + min order check
	if req.DeliveryType == "DELIVERY" {
		switch zoneIdx {
		case 0: // olmaliq.json
			req.DeliveryPrice = 0
			// if productsTotal < olmaliqMin {
			// 	return "", "", structs.ErrMinOrder{
			// 		ZoneKey: "OLMALIQ",
			// 		Min:     olmaliqMin,
			// 		Current: productsTotal,
			// 	}
			// }

		case 1: // ohangaron.json
			req.DeliveryPrice = 25000
			if productsTotal < ohangaronMin {
				return "", "", structs.ErrMinOrder{
					ZoneKey: "OHANGARON",
					Min:     ohangaronMin,
					Current: productsTotal,
				}
			}

		default:
			return "", "", structs.ErrOutOfDeliveryZone
		}
	}

	// 5) Create order in DB (repo status/paysni payment method bo'yicha o'zi qo'yadi)
	id, err := s.orderRepo.Create(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.Create", zap.Error(err))
		return "", "", err
	}

	ord, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByID after Create", zap.Error(err))
		return "", "", err
	}

	// 6) CASH bo'lsa link yo'q
	if req.PaymentMethod == "CASH" {
		return "", id, nil
	}

	// 7) Online bo'lsa (CLICK/PAYME) payment link yaratamiz (agar oldin saqlangan bo'lsa qaytaramiz)
	if strings.TrimSpace(ord.Order.PaymentUrl) != "" {
		return ord.Order.PaymentUrl, id, nil
	}

	switch req.PaymentMethod {
	case "CLICK":
		serviceId := strings.TrimSpace(os.Getenv("CLICK_SERVICE_ID"))
		merchantId := strings.TrimSpace(os.Getenv("CLICK_MERCHANT_ID"))
		if serviceId == "" || merchantId == "" {
			return "", id, fmt.Errorf("CLICK_SERVICE_ID yoki CLICK_MERCHANT_ID env not found")
		}

		merchantTransID := cast.ToString(ord.Order.OrderNumber)
		amountInt := ord.Order.TotalPrice

		// prepare
		prep, err := s.clickSvc.CheckoutPrepare(ctx, structs.CheckoutPrepareRequest{
			ServiceID:        serviceId,
			MerchantID:       merchantId,
			TransactionParam: merchantTransID,
			Amount:           float64(amountInt),
			Description:      fmt.Sprintf("Order #%d", ord.Order.OrderNumber),
		})
		if err != nil {
			return "", id, fmt.Errorf("click checkout/prepare failed: %w", err)
		}

		// orderga click info yozib qo'yish (request_id / transaction_param)
		_ = s.orderRepo.UpdateClickInfo(ctx, id, prep.RequestId, merchantTransID)

		sid := cast.ToInt64(serviceId)
		payURL := s.BuildClickPayURL(sid, merchantId, amountInt, merchantTransID, "")
		if payURL == "" {
			return "", id, fmt.Errorf("click pay url empty")
		}

		if err := s.orderRepo.AddLink(ctx, payURL, id); err != nil {
			return "", id, err
		}
		return payURL, id, nil

	case "PAYME":
		merchantID := strings.TrimSpace(os.Getenv("PAYME_KASSA_ID"))
		if merchantID == "" {
			return "", id, fmt.Errorf("PAYME_KASSA_ID env not found")
		}

		amountTiyin := ord.Order.TotalPrice * 100
		transactionParam := cast.ToString(ord.Order.OrderNumber)

		payURL, err := s.paymeSvc.BuildPaymeCheckoutURL(merchantID, transactionParam, amountTiyin)
		if err != nil {
			return "", id, fmt.Errorf("build payme checkout url failed: %w", err)
		}

		if err := s.orderRepo.AddLink(ctx, payURL, id); err != nil {
			return "", id, err
		}
		return payURL, id, nil

	default:
		return "", id, structs.ErrBadRequest
	}
}

func (s *service) ConfirmByOperator(ctx context.Context, orderID string) error {
	return s.sendToIikoIfAllowed(ctx, orderID)
}

func (s *service) sendToIikoIfAllowed(ctx context.Context, orderID string) error {
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
	s.notifyOrderStatusIfNeeded(ctx, req.OrderId, "WAITING_PAYMENT")
	switch pStatus {
	case "PAID":
		if paymentMethod == "CLICK" || paymentMethod == "PAYME" {
			if deliveryType == "DELIVERY" && ord.Order.Address == nil {
				return fmt.Errorf("paid but address missing")
			}
			go func(orderID string) {
				_ = s.sendToIikoIfAllowed(context.Background(), orderID)
			}(req.OrderId)
			return nil
		}
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

	// ✅ FIX: avval DB status update qilamiz (COOKING ham)
	if err := s.orderRepo.UpdateStatus(ctx, req); err != nil {
		return err
	}
	s.notifyOrderStatusIfNeeded(ctx, req.OrderId, st)
	// COOKING bo'lsa iiko'ga yuborishni ham urinib ko'ramiz
	if st == "COOKING" {
		if err := s.sendToIikoIfAllowed(ctx, req.OrderId); err != nil {
			return err
		}
	}
	if strings.Contains(st, "CLOSE") || strings.Contains(st, "COMPLETE") {
		if err := s.orderRepo.UpdatePaymentStatus(ctx, structs.UpdateStatus{
			OrderId: req.OrderId,
			Status:  "PAID",
		}); err != nil {
			s.logger.Error(ctx, "can't update payment status err", zap.Error(err))
			return err
		}
	}

	return nil
}

func (s *service) BuildClickPayURL(serviceID int64, merchantID string, amountInt int64, orderID, returnURL string) string {
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
		paymentKind = "Card"       // ✅ MUHIM
		processedExternally = true // qolsa ham bo‘ladi
		paymentTypeID = paymentClickID

	case "PAYME":
		paymentKind = "Card" // ✅ MUHIM
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

// --- NOTIFY PART ---

func (s *service) HandleIikoDeliveryOrderUpdate(ctx context.Context, evt structs.IikoWebhookEvent) error {
	s.logger.Info(ctx, "IIKO webhook handling",
		zap.String("eventType", evt.EventType),
		zap.String("externalNumber", evt.EventInfo.ExternalNumber),
		zap.String("creationStatus", evt.EventInfo.CreationStatus),
	)

	if strings.ToUpper(strings.TrimSpace(evt.EventType)) != "DELIVERYORDERUPDATE" {
		return nil
	}

	ext := strings.TrimSpace(evt.EventInfo.ExternalNumber)
	if ext == "" {
		s.logger.Info(ctx, "IIKO webhook ignored (no externalNumber)",
			zap.String("eventType", evt.EventType),
			zap.String("iiko_id", evt.EventInfo.ID),
			zap.String("pos_id", evt.EventInfo.PosID),
			zap.String("creationStatus", evt.EventInfo.CreationStatus),
		)
		return nil
	}

	num, err := strconv.ParseInt(ext, 10, 64)
	if err != nil {
		s.logger.Warn(ctx, "IIKO webhook bad externalNumber", zap.String("externalNumber", ext), zap.Error(err))
		return nil
	}
	ord, err := s.orderRepo.GetByOrderNumber(ctx, num)
	if err != nil {
		s.logger.Error(ctx, "IIKO webhook GetByOrderNumber failed",
			zap.Int64("orderNumber", num),
			zap.Error(err),
		)
		return err
	}

	if strings.ToUpper(strings.TrimSpace(evt.EventInfo.CreationStatus)) != "SUCCESS" {
		s.logger.Warn(ctx, "IIKO webhook creationStatus not SUCCESS -> REJECTED",
			zap.String("creationStatus", evt.EventInfo.CreationStatus),
			zap.String("orderId", ord.ID),
		)
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.ID, Status: "REJECTED"})
		return nil
	}

	// iiko meta
	if err := s.orderRepo.UpdateIikoMeta(ctx, ord.ID, evt.EventInfo.ID, evt.EventInfo.PosID, evt.CorrelationId); err != nil {
		s.logger.Error(ctx, "IIKO webhook UpdateIikoMeta failed",
			zap.String("orderId", ord.ID),
			zap.Error(err),
		)
	}

	iikoStatus := extractIikoOrderStatus(evt.EventInfo.Order)
	s.logger.Info(ctx, "IIKO webhook extracted status",
		zap.String("externalNumber", evt.EventInfo.ExternalNumber),
		zap.String("iikoStatus", iikoStatus),
		zap.Int("orderRawLen", len(evt.EventInfo.Order)),
		zap.String("orderRawHead", func() string {
			raw := string(evt.EventInfo.Order)
			if len(raw) > 600 {
				return raw[:600]
			}
			return raw
		}()),
	)

	if iikoStatus == "" {
		// vaqtincha: raw orderni ham qisqartirib log qiling
		raw := string(evt.EventInfo.Order)
		if len(raw) > 500 {
			raw = raw[:500] + "..."
		}
		s.logger.Warn(ctx, "iiko order has no status", zap.String("orderRaw", raw))
		return nil
	}
	newStatus := mapIikoStatusToOurStatus(iikoStatus)
	if newStatus == "" {
		return nil
	}
	if ord.DeliveryType == "PICKUP" && newStatus == "ON_THE_WAY" {
		newStatus = "READY_FOR_PICKUP"
	}

	if err := s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.ID, Status: newStatus}); err != nil {
		s.logger.Error(ctx, "IIKO webhook UpdateStatus failed",
			zap.String("orderId", ord.ID),
			zap.String("status", newStatus),
			zap.Error(err),
		)
		return err
	}

	final := newStatus == "COMPLETED" || newStatus == "DELIVERED"
	pm := strings.ToUpper(strings.TrimSpace(ord.PaymentMethod))
	if final && (pm == "CASH" || pm == "NAQD") {
		_ = s.orderRepo.UpdatePaymentStatus(ctx, structs.UpdateStatus{
			OrderId: ord.ID,
			Status:  "PAID",
		})
	}

	s.notifyOrderStatusIfNeeded(ctx, ord.ID, newStatus)
	return nil
}

func (s *service) notifyOrderStatusIfNeeded(ctx context.Context, orderID string, newStatus string) {
	if s.bot == nil {
		return
	}

	s.logger.Info(ctx, "notifyOrderStatusIfNeeded called",
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
				ID:          orderID,
				Status:      newStatus,
				OrderNumber: int64(target.OrderNumber),
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

func (s *service) HandleIikoDeliveryOrderError(ctx context.Context, evt structs.IikoWebhookEvent) error {
	if strings.ToUpper(strings.TrimSpace(evt.EventType)) != "DELIVERYORDERERROR" {
		return nil
	}

	ext := strings.TrimSpace(evt.EventInfo.ExternalNumber)
	if ext == "" {
		s.logger.Info(ctx, "IIKO webhook ignored (no externalNumber)",
			zap.String("eventType", evt.EventType),
			zap.String("iiko_id", evt.EventInfo.ID),
			zap.String("pos_id", evt.EventInfo.PosID),
			zap.String("creationStatus", evt.EventInfo.CreationStatus),
		)
		return nil
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
	_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: ord.ID, Status: "REJECTED"})
	s.notifyOrderStatusIfNeeded(ctx, ord.ID, "REJECTED")

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
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}

	// 1) top-level
	if s := pickStatusValue(m["status"]); s != "" {
		return s
	}
	if s := pickStatusValue(m["deliveryStatus"]); s != "" {
		return s
	}
	if s := pickStatusValue(m["state"]); s != "" {
		return s
	}

	// 2) ba’zi payloadlarda nested bo‘lishi mumkin
	if om, ok := m["order"].(map[string]any); ok {
		if s := pickStatusValue(om["status"]); s != "" {
			return s
		}
		if s := pickStatusValue(om["deliveryStatus"]); s != "" {
			return s
		}
	}
	if dm, ok := m["delivery"].(map[string]any); ok {
		if s := pickStatusValue(dm["status"]); s != "" {
			return s
		}
		if s := pickStatusValue(dm["deliveryStatus"]); s != "" {
			return s
		}
	}

	return ""
}

func pickStatusValue(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case map[string]any:
		// iiko ko‘p ishlatadigan nomlar
		for _, k := range []string{"name", "value", "code", "key", "status"} {
			if s, ok := x[k].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
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
	case strings.Contains(s, "WAITING"):
		return "ON_THE_WAY"
	case strings.Contains(s, "ONWAY"):
		return "DELIVERED"
	case strings.Contains(s, "WAITING"):
		return "READY_FOR_PICKUP"
	case strings.Contains(s, "CLOSE") || strings.Contains(s, "COMPLETE"):
		return "COMPLETED"
	default:
		return ""
	}
}

// func mapIikoStatusToOurStatus(iikoStatus string) string {
// 	s := strings.ToUpper(strings.TrimSpace(iikoStatus))
// 	switch {
// 	case strings.Contains(s, "CANCEL"):
// 		return "CANCELLED"
// 	case strings.Contains(s, "REJECT"):
// 		return "REJECTED"
// 	case strings.Contains(s, "DELIVERED"):
// 		return "DELIVERED"
// 	case strings.Contains(s, "COOK"):
// 		return "COOKING"
// 	case strings.Contains(s, "READY"):
// 		return "READY_FOR_PICKUP"
// 	case strings.Contains(s, "WAY") || strings.Contains(s, "COURIER") || strings.Contains(s, "DELIVERY") || strings.Contains(s, "ONWAY"):
// 		return "ON_THE_WAY"
// 	case strings.Contains(s, "CLOSE") || strings.Contains(s, "COMPLETE"):
// 		return "COMPLETED"
// 	default:
// 		return ""
// 	}
// }

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

func (s *service) DeliveryMapFound(ctx context.Context, req structs.MapFoundRequest) (int64, bool, error) {
	ok, idx, err := s.zones.ContainsAnyWithIndex(req.Lat, req.Lng)
	if err != nil {
		return 0, false, fmt.Errorf("zone check failed: %w", err)
	}
	if !ok {
		return 0, false, structs.ErrOutOfDeliveryZone
	}
	var (
		price     int64
		available bool
	)
	switch idx {
	case 0: // olmaliq.json
		price = 0
		available = true
	case 1: // ohangaron.json
		price = 25000
		available = true
	default:
		return 0, false, structs.ErrOutOfDeliveryZone
	}
	return price, available, nil
}
func (s *service) publishUpsertToAdmins(dto structs.OrderDTO) {
	evt := structs.Event{
		Type: structs.EventOrderUpsert,
		Payload: structs.OrderUpsertPayload{
			Order: dto,
		},
	}
	s.hub.BroadcastToAdmins(evt)
}

func mapOrdToDTO(ord structs.GetListPrimaryKeyResponse) structs.OrderDTO {
	return structs.OrderDTO{
		ID:            ord.Order.ID,
		TgID:          ord.Order.TgID,
		Phone:         ord.Phone,
		DeliveryType:  ord.Order.DeliveryType,
		PaymentMethod: ord.Order.PaymentMethod,
		PaymentStatus: ord.Order.PaymentStatus,
		Status:        ord.Order.Status, // sizda field nomi qanday bo'lsa shunga moslang
		TotalPrice:    ord.Order.TotalPrice,
		TotalCount:    ord.Order.TotalCount,
		DeliveryPrice: ord.Order.DeliveryPrice,
		OrderNumber:   ord.Order.OrderNumber,
		PaymentUrl:    ord.Order.PaymentUrl,
		CreatedAt:     ord.Order.CreatedAt,
		UpdateAt:      ord.Order.UpdateAt,
	}
}

func (s *service) TrySendToIiko(ctx context.Context, orderID string) error {
	return s.sendToIikoIfAllowed(ctx, orderID)
}

func (s *service) resolveOrderForIikoWebhook(ctx context.Context, evt structs.IikoWebhookEvent) (structs.Order, bool) {
	ext := strings.TrimSpace(evt.EventInfo.ExternalNumber)

	// 1) externalNumber bor bo‘lsa:
	if ext != "" {
		// 1a) int bo‘lsa -> order_number
		if num, err := strconv.ParseInt(ext, 10, 64); err == nil {
			ord, e := s.orderRepo.GetByOrderNumber(ctx, num)
			if e == nil {
				return ord, true
			}
			s.logger.Warn(ctx, "webhook: order not found by orderNumber",
				zap.Int64("orderNumber", num),
				zap.Error(e),
			)
		} else {
			// 1b) uuid bo‘lsa -> order_id
			pk, e := s.orderRepo.GetByID(ctx, ext)
			if e == nil {
				return pk.Order, true
			}
			s.logger.Warn(ctx, "webhook: order not found by orderID(externalNumber)",
				zap.String("externalNumber", ext),
				zap.Error(e),
			)
		}
	}

	// 2) externalNumber yo‘q bo‘lsa (yoki topilmasa) -> iiko order id bo‘yicha fallback
	iikoID := strings.TrimSpace(evt.EventInfo.ID)
	if iikoID != "" {
		ord, e := s.orderRepo.GetByIikoOrderID(ctx, iikoID)
		if e == nil {
			return ord, true
		}
	}

	return structs.Order{}, false
}
