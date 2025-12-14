package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	clickrepo "sushitana/pkg/repository/postgres/payment_repo/click_repo"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(NewUsecase)

type UsecaseParams struct {
	fx.In
	Logger    logger.Logger
	OrderRepo orderrepo.Repo
	ClickRepo clickrepo.Repo
}

type Usecase interface {
	Prepare(ctx context.Context, req structs.PrepareRequest) (merchantPrepareID int64, errCode int, errNote string)
	Complete(ctx context.Context, req structs.CompleteRequest) (merchantConfirmID int64, errCode int, errNote string)
	Cancel(ctx context.Context, req structs.CompleteRequest) (errCode int, errNote string)
}

type usecase struct {
	logger    logger.Logger
	orderRepo orderrepo.Repo
	clickRepo clickrepo.Repo
	now       func() time.Time
}

func NewUsecase(p UsecaseParams) Usecase {
	return &usecase{
		logger:    p.Logger,
		orderRepo: p.OrderRepo,
		clickRepo: p.ClickRepo,
		now:       time.Now,
	}
}

func (u *usecase) getOrderByMTI(ctx context.Context, merchantTransID string) (structs.Order, error) {
	merchantTransID = strings.TrimSpace(merchantTransID)
	if merchantTransID == "" {
		return structs.Order{}, structs.ErrNotFound
	}

	o, err := u.orderRepo.GetByMerchantTransId(ctx, merchantTransID)
	if err == nil {
		return o, nil
	}
	if !errors.Is(err, structs.ErrNotFound) {
		return structs.Order{}, err
	}

	n, err := strconv.ParseInt(merchantTransID, 10, 64)
	if err != nil {
		return structs.Order{}, structs.ErrNotFound
	}

	o, err = u.orderRepo.GetByOrderNumber(ctx, n)
	if err == nil {
		return o, nil
	}
	return structs.Order{}, err
}

// Sizda real summa qayerdan olinadi — shu joyni moslang.
// Hozircha callbackdagi amountni “to‘g‘ri” deb qabul qilmaymiz, orderdan hisoblab tekshiramiz.
func (u *usecase) expectedAmount(order structs.Order) (string, error) {
	// TODO: sizning real logikangiz:
	// total := order.TotalPrice + order.DeliveryPrice
	// return strconv.FormatInt(total, 10), nil

	// Hozircha: implement qilmagansiz — shuni majburan moslang

	return "1000", errors.New("expectedAmount not implemented")
}

// =========================
// Usecase methods
// =========================

// Prepare (action=0):
// - order borligini tekshir
// - amount mosligini tekshir
// - idempotency: click_trans_id bo‘yicha oldin yaratilgan bo‘lsa o‘shani qaytar
// - bo‘lmasa attempt yaratib, attempt.ID ni merchant_prepare_id qilib qaytar
func (u *usecase) Prepare(ctx context.Context, req structs.PrepareRequest) (int64, int, string) {
	order, err := u.getOrderByMTI(ctx, req.MerchantTransID)
	if err != nil {
		return 0, structs.ErrUserOrOrderMissing, "Order not found"
	}

	// Idempotency: click_trans_id unique
	if prev, err := u.clickRepo.GetByClickTransID(ctx, req.ClickTransID); err == nil {
		// agar avvalgi attempt PAID bo‘lsa — qayta prepare kelsa ham OK qaytaramiz
		return prev.MerchantPrepareID, structs.ErrSuccess, "Success"
	}

	// Amount check (majburiy tavsiya)
	exp, aerr := u.expectedAmount(order)
	if aerr == nil && strings.TrimSpace(exp) != "" {
		if strings.TrimSpace(exp) != strings.TrimSpace(req.Amount) {
			return 0, structs.ErrIncorrectAmount, "Incorrect parameter amount"
		}
	} else {
		// Agar hozircha expectedAmount qilinmagan bo‘lsa:
		// u.logger.Warn(...) qilib qo‘ying va vaqtincha tekshiruvni o‘chirib turing.
		u.logger.Warn(ctx, "expectedAmount not implemented; skipping amount check",
			zap.String("merchant_trans_id", req.MerchantTransID),
			zap.String("amount", req.Amount),
		)
	}

	newID, err := u.clickRepo.Create(ctx, structs.Invoice{
		MerchantTransID: req.MerchantTransID,
		ClickTransID:    req.ClickTransID,
		ClickPaydocID:   req.ClickPaydocID,
		Amount:          req.Amount,
		Status:          "PENDING",
		CreatedAt:       u.now(),
		UpdatedAt:       u.now(),
	})
	if err != nil {
		u.logger.Error(ctx, "clickRepo.CreateAttempt", zap.Error(err))
		return 0, structs.ErrFailedToUpdate, "Failed to update user data"
	}

	return newID, structs.ErrSuccess, "Success"
}

// Complete (action=1, error=0):
// - attempt(merchant_prepare_id) borligini tekshir
// - idempotency: attempt allaqachon PAID bo‘lsa, o‘sha confirm_id bilan success qaytar
// - amount mosligini tekshir
// - orderni PAID qil
// - attemptni PAID qil (merchant_confirm_id ber)
func (u *usecase) Complete(ctx context.Context, req structs.CompleteRequest) (int64, int, string) {
	attempt, err := u.clickRepo.GetByPrepareID(ctx, req.MerchantPrepareID)
	if err != nil {
		return 0, structs.ErrTransactionMissing, "Transaction not found"
	}

	// Idempotency: qayta Complete kelsa
	if attempt.Status == "PAID" && attempt.MerchantPrepareID != 0 {
		return attempt.MerchantPrepareID, structs.ErrSuccess, "Success"
	}

	order, err := u.getOrderByMTI(ctx, req.MerchantTransID)
	if err != nil {
		return 0, structs.ErrUserOrOrderMissing, "Order not found"
	}

	// amount check (xuddi Prepare kabi)
	exp, aerr := u.expectedAmount(order)
	if aerr == nil && strings.TrimSpace(exp) != "" {
		if strings.TrimSpace(exp) != strings.TrimSpace(req.Amount) {
			return 0, structs.ErrIncorrectAmount, "Incorrect parameter amount"
		}
	} else {
		u.logger.Warn(ctx, "expectedAmount not implemented; skipping amount check",
			zap.String("merchant_trans_id", req.MerchantTransID),
			zap.String("amount", req.Amount),
		)
	}

	// merchant_confirm_id: sizning internal confirm id’ingiz (odatda int64)
	// Eng oddiy: prepareID’ni confirmID sifatida ishlatish (stabil, idempotent).
	confirmID := req.MerchantPrepareID

	if err := u.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
		OrderId: order.ID,
		Status:  attempt.Status,
	}); err != nil {
		u.logger.Error(ctx, "orderRepo.SetPaid", zap.Error(err), zap.String("order_id", order.ID))
		return 0, structs.ErrFailedToUpdate, "Failed to update user data"
	}

	return confirmID, structs.ErrSuccess, "Success"
}

// Cancel (action=1, error != 0):
// - attempt topiladi
// - agar PAID bo‘lsa: -4 (yoki success qaytarib qo‘yish ham mumkin)
// - bo‘lmasa attempt CANCELLED + order UNPAID/CANCELLED
func (u *usecase) Cancel(ctx context.Context, req structs.CompleteRequest) (int, string) {
	attempt, err := u.clickRepo.GetByPrepareID(ctx, req.MerchantPrepareID)
	if err != nil {
		return structs.ErrTransactionMissing, "Transaction not found"
	}

	if attempt.Status == "PAID" {
		return structs.ErrAlreadyPaid, "Already paid"
	}

	// order’ni qay holatga tushirasiz — sizning biznes flow’ga bog‘liq
	order, oerr := u.getOrderByMTI(ctx, req.MerchantTransID)
	if oerr == nil {
		_ = u.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: order.ID,
			Status:  "UNPAID",
		})
	}

	// Click protokolda Cancel’ning o‘zi handler darajasida -9 bilan qaytariladi (siz shunday qilyapsiz)
	return structs.ErrSuccess, "Success"
}
