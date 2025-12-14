package order

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
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
	// case "CLICK":
	// 	serviceId := os.Getenv("CLICK_SERVICE_ID")
	// 	merchantId := os.Getenv("CLICK_MERCHANT_ID")
	// 	if serviceId == "" || merchantId == "" {
	// 		return "", fmt.Errorf("CLICK_SERVICE_ID yoki CLICK_MERCHANT_ID env yo‘q")
	// 	}

	// 	// merchant_trans_id bo‘ladigan qiymat (unikal bo‘lsin)
	// 	merchantTransID := cast.ToString(order.Order.OrderNumber)

	// 	// MUHIM: bu "pul summasi" bo‘lishi kerak (count emas!)
	// 	// masalan: total = order total + delivery
	// 	// amountInt := int64(order.Order.TotalPrice) // <-- sizdagi real fieldga moslang
	// 	amountInt := 1000
	// 	amount := float64(amountInt)

	// 	// 1) internal/checkout/prepare
	// 	_, err := s.clickSvc.CheckoutPrepare(ctx, structs.CheckoutPrepareRequest{
	// 		ServiceID:        serviceId,
	// 		MerchantID:       merchantId,
	// 		TransactionParam: merchantTransID, // => callback’da merchant_trans_id bo‘lib keladi
	// 		Amount:           amount,
	// 		ReturnUrl:        "order",
	// 		Description:      fmt.Sprintf("Order #%s", merchantTransID),
	// 	})
	// 	if err != nil {
	// 		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: order.Order.ID, Status: "UNPAID"})
	// 		return "", fmt.Errorf("click checkout/prepare failed: %w", err)
	// 	}

	// 	// 2) internal/checkout/invoice (SMS yuboradi)
	// 	// invResp, err := s.clickSvc.CheckoutInvoice(ctx, structs.CheckoutInvoiceRequest{
	// 	// 	RequestId:   prep.RequestId,
	// 	// 	PhoneNumber: order.Phone, // formatni Click talabiga moslab yuboring
	// 	// })
	// 	// if err != nil {
	// 	// 	_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: order.Order.ID, Status: "UNPAID"})
	// 	// 	return "", fmt.Errorf("click checkout/invoice failed: %w", err)
	// 	// }

	// 	// 3) invoices jadvaliga yozib qo‘ying
	// 	inv := structs.Invoice{
	// 		// ClickInvoiceID:  invResp.InvoiceId,
	// 		MerchantTransID: merchantTransID,
	// 		OrderID:         sql.NullString{String: order.Order.ID, Valid: true},
	// 		TgID:            sql.NullInt64{Int64: order.Order.TgID, Valid: true},
	// 		CustomerPhone:   sql.NullString{String: order.Phone, Valid: true},
	// 		Amount:          cast.ToString(amountInt),
	// 		Currency:        "UZS",
	// 		Status:          "WAITING_PAYMENT",
	// 	}
	// 	_ = s.clickRepo.Create(ctx, inv)

	// 	_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
	// 		OrderId: order.Order.ID,
	// 		Status:  "WAITING_PAYMENT",
	// 	})

	// 	// bu flow’da siz link qaytarmaysiz, SMS ketadi
	// 	payURL = BuildClickPayURL(cast.ToInt64(serviceId), merchantId, int64(amountInt), merchantTransID, "order")
	// 	return payURL, nil

	case "CLICK":
		// 1) order status -> WAITING_PAYMENT
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "WAITING_PAYMENT",
		})

		// 2) merchant_trans_id (unikal bo‘lsin, callback’da shu keladi)
		// tavsiya: order.Order.ID (uuid) yoki order.Order.OrderNumber
		merchantTransID := cast.ToString(order.Order.OrderNumber)

		// 3) amount (UZS) — real summaga moslang (total + delivery)
		// TODO: sizdagi real fieldga moslab qo‘ying
		amountInt := int64(1000)
		amountStr := strconv.FormatInt(amountInt, 10)

		// 4) Shop redirect URL yasash (SMS emas)
		// ReturnURL ixtiyoriy: webapp route yoki saytga qaytish URL
		returnURL := os.Getenv("CLICK_RETURN_URL") // bo‘lmasa "" qolsin

		payURL, err = s.shopSvc.BuildPayURL(ctx, structs.PayLinkParams{
			// ServiceID/MerchantID bo‘sh qoldirsangiz shopSvc envdan oladi (siz shunaqa qilgansiz)
			Amount:           amountStr,
			TransactionParam: merchantTransID,
			ReturnURL:        returnURL,
		})
		if err != nil {
			s.logger.Error(ctx, "->shopSvc.BuildPayURL failed", zap.Error(err), zap.String("order_id", id))
			_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{OrderId: order.Order.ID, Status: "UNPAID"})
			return "", fmt.Errorf("build click pay url failed: %w", err)
		}

		// (ixtiyoriy) attemptni DBga yozish: click_trans_id callbackda keladi, hozircha faqat MTI/amount/status saqlang
		// _ = s.clickRepo.Create(ctx, ...)

		return payURL, nil
	case "PAYME":
		// 1) order status -> WAITING_PAYMENT
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.Order.ID,
			Status:  "WAITING_PAYMENT",
		})

		// 2) payme checkout link yasab qaytarish
		merchantID := os.Getenv("PAYME_KASSA_ID") // kassangiz (merchant) id
		amountTiyin := int64(1000 * 100)          // som -> tiyin ---------------------------
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
	v.Set("amount", fmt.Sprintf("%d.00", amountInt)) // N.NN format :contentReference[oaicite:3]{index=3}
	v.Set("transaction_param", orderID)              // => merchant_trans_id :contentReference[oaicite:4]{index=4}
	if returnURL != "" {
		v.Set("return_url", returnURL)
	}
	return "https://my.click.uz/services/pay?" + v.Encode()
}
