package payme

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	paymerepo "sushitana/pkg/repository/postgres/payment_repo/payme_repo"
	"time"

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
		Logger    logger.Logger
		OrderRepo orderrepo.Repo
		PaymeRepo paymerepo.Repo
	}
	Service interface {
		CheckPerformTransaction(ctx context.Context, p structs.PaymeCheckPerformParams) (structs.PaymeCheckPerformResult, structs.RPCError)
		CreateTransaction(ctx context.Context, p structs.PaymeCreateParams) (structs.PaymeCreateResult, structs.RPCError)
		PerformTransaction(ctx context.Context, p structs.PaymePerformParams) (structs.PaymePerformResult, structs.RPCError)
		CancelTransaction(ctx context.Context, p structs.PaymeCancelParams) (structs.PaymeCancelResult, structs.RPCError)
		CheckTransaction(ctx context.Context, p structs.PaymeCheckParams) (structs.PaymeCheckResult, structs.RPCError)
		GetStatement(ctx context.Context, p structs.PaymeStatementParams) (structs.PaymeStatementResult, structs.RPCError)
		BuildPaymeCheckoutURL(merchantID string, orderID string, amountTiyin int64) (string, error)
	}
	service struct {
		logger    logger.Logger
		orderRepo orderrepo.Repo
		paymeRepo paymerepo.Repo
	}
)

func New(p Params) Service {
	return &service{
		logger:    p.Logger,
		orderRepo: p.OrderRepo,
		paymeRepo: p.PaymeRepo,
	}
}

func nowMs() int64 { return time.Now().UnixMilli() }

func paymeMsg(ru, uz, en string) structs.PaymeMessage {
	return structs.PaymeMessage{Ru: ru, Uz: uz, En: en}
}

func rpcErr(code int, ru, uz, en string, data any) structs.RPCError {
	return structs.RPCError{
		Code:    code,
		Message: paymeMsg(ru, uz, en),
		Data:    data,
	}
}

func tiyinToSomString(t int64) string {
	neg := t < 0
	if neg {
		t = -t
	}
	som := t / 100
	tiyin := t % 100
	out := fmt.Sprintf("%d.%02d", som, tiyin)
	if neg {
		return "-" + out
	}
	return out
}

func somStringToTiyin(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}

	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	}

	parts := strings.SplitN(s, ".", 2)
	som, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}

	var tiyin int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) == 0 {
			frac = "00"
		} else if len(frac) == 1 {
			frac = frac + "0"
		} else if len(frac) > 2 {
			frac = frac[:2]
		}
		tiyin, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, err
		}
		if tiyin < 0 || tiyin > 99 {
			return 0, fmt.Errorf("invalid tiyin")
		}
	}

	out := som*100 + tiyin
	if neg {
		out = -out
	}
	return out, nil
}

func parseOrderID(a structs.Account) (string, bool) {
	if a.OrderID != "" {
		return a.OrderID, true
	}
	if a.ID != "" {
		return a.ID, true
	}
	return "", false
}

func expectedAmountTiyinFromOrderTotal(total any) int64 {
	// sizda TotalPrice float/int bo‘lishi mumkin — float rounding muammo bo‘lmasin
	// NOTE: total ni float64 ga o‘tkazish uchun fmt emas, direct cast ishlatamiz:
	// bu yerda biz “best-effort” qilamiz: total string emas deb hisoblaymiz.
	switch v := total.(type) {
	case float64:
		return int64(math.Round(v * 100))
	case float32:
		return int64(math.Round(float64(v) * 100))
	case int:
		return int64(v) * 100
	case int64:
		return v * 100
	case int32:
		return int64(v) * 100
	case uint64:
		return int64(v) * 100
	case uint:
		return int64(v) * 100
	default:
		return 0
	}
}

func (s *service) CheckPerformTransaction(ctx context.Context, p structs.PaymeCheckPerformParams) (structs.PaymeCheckPerformResult, structs.RPCError) {
	orderID, ok := parseOrderID(p.Account)
	if !ok {
		return structs.PaymeCheckPerformResult{Allow: false}, rpcErr(
			-31050,
			"Неверный счет",
			"Hisob noto‘g‘ri",
			"Invalid account",
			"order_id",
		)
	}

	ord, err := s.orderRepo.GetByOrderNumber(ctx, cast.ToInt64(orderID))
	if err != nil {
		return structs.PaymeCheckPerformResult{Allow: false}, rpcErr(
			-31050,
			"Счет не найден",
			"Hisob topilmadi",
			"Account not found",
			"order_id",
		)
	}

	if ord.Status != "WAITING_PAYMENT" {
		return structs.PaymeCheckPerformResult{Allow: false}, rpcErr(
			-31052,
			"Операция недоступна",
			"Amalga ruxsat yo‘q",
			"Operation not allowed",
			"order_status",
		)
	}

	expected := expectedAmountTiyinFromOrderTotal(any(ord.TotalPrice))
	if expected == 0 {
		expected = int64(math.Round(float64(ord.TotalPrice) * 100)) // ✅ *100
	}

	if p.Amount != expected {
		return structs.PaymeCheckPerformResult{Allow: false}, rpcErr(
			-31001,
			"Неверная сумма",
			"Noto‘g‘ri summa",
			"Incorrect amount",
			"amount",
		)
	}

	return structs.PaymeCheckPerformResult{Allow: true}, structs.RPCError{}
}
func (s *service) CreateTransaction(ctx context.Context, p structs.PaymeCreateParams) (structs.PaymeCreateResult, structs.RPCError) {
	orderID, ok := parseOrderID(p.Account)
	if !ok {
		return structs.PaymeCreateResult{}, rpcErr(
			-31050,
			"Неверный счет",
			"Hisob noto‘g‘ri",
			"Invalid account",
			"order_id",
		)
	}

	ord, err := s.orderRepo.GetByOrderNumber(ctx, cast.ToInt64(orderID))
	if err != nil {
		return structs.PaymeCreateResult{}, rpcErr(
			-31050,
			"Счет не найден",
			"Hisob topilmadi",
			"Account not found",
			"order_id",
		)
	}

	if ord.Status != "WAITING_PAYMENT" {
		return structs.PaymeCreateResult{}, rpcErr(
			-31052,
			"Операция недоступна",
			"Amalga ruxsat yo‘q",
			"Operation not allowed",
			"order_status",
		)
	}

	expected := expectedAmountTiyinFromOrderTotal(any(ord.TotalPrice))
	if expected == 0 {
		expected = int64(math.Round(float64(ord.TotalPrice)))
	}
	if p.Amount != expected {
		return structs.PaymeCreateResult{}, rpcErr(
			-31001,
			"Неверная сумма",
			"Noto‘g‘ri summa",
			"Incorrect amount",
			"amount",
		)
	}

	existing, e := s.paymeRepo.GetByPaycomTransactionID(ctx, p.Id)
	if e == nil {
		return structs.PaymeCreateResult{
			Transaction: existing.PaycomTransactionID,
			State:       existing.State,
			CreateTime:  existing.CreatedTime,
		}, structs.RPCError{}
	}

	amountSom := tiyinToSomString(p.Amount)

	tx, err := s.paymeRepo.Create(ctx, ord.ID, p.Id, amountSom, p.Time)
	if err != nil {
		if strings.Contains(err.Error(), "payme_one_active_per_order_uq") &&
			strings.Contains(err.Error(), "SQLSTATE 23505") {

			ex, e2 := s.paymeRepo.GetByPaycomTransactionID(ctx, p.Id)
			if e2 == nil {
				return structs.PaymeCreateResult{
					Transaction: ex.PaycomTransactionID,
					State:       ex.State,
					CreateTime:  ex.CreatedTime,
				}, structs.RPCError{}
			}

			return structs.PaymeCreateResult{}, rpcErr(
				-31052,
				"Транзакция уже существует",
				"Tranzaksiya allaqachon mavjud",
				"Transaction already exists",
				"transaction",
			)
		}

		s.logger.Error(ctx, "payme CreateTransaction repo.Create failed", zap.Error(err))
		return structs.PaymeCreateResult{}, rpcErr(
			-32400,
			"Внутренняя ошибка",
			"Ichki xato",
			"Internal error",
			nil,
		)
	}

	return structs.PaymeCreateResult{
		Transaction: tx.PaycomTransactionID,
		State:       tx.State,
		CreateTime:  tx.CreatedTime,
	}, structs.RPCError{}
}

func (s *service) PerformTransaction(ctx context.Context, p structs.PaymePerformParams) (structs.PaymePerformResult, structs.RPCError) {
	tx, err := s.paymeRepo.GetByPaycomTransactionID(ctx, p.Id)
	if err != nil {
		return structs.PaymePerformResult{}, rpcErr(
			-31003,
			"Транзакция не найдена",
			"Tranzaksiya topilmadi",
			"Transaction not found",
			"transaction")
	}

	if tx.State == paymerepo.StatePerformed {
		return structs.PaymePerformResult{
			Transaction: tx.PaycomTransactionID,
			State:       tx.State,
			PerformTime: tx.PerformTime.Int64,
		}, structs.RPCError{}
	}

	if tx.State < 0 {
		return structs.PaymePerformResult{}, rpcErr(
			-31008,
			"Транзакция отменена",
			"Tranzaksiya bekor qilingan",
			"Transaction canceled",
			"state")
	}

	updated, err := s.paymeRepo.MarkPerformed(ctx, p.Id, nowMs())
	if err != nil {
		s.logger.Error(ctx, "payme MarkPerformed failed", zap.Error(err))
		return structs.PaymePerformResult{}, rpcErr(-32400, "Внутренняя ошибка", "Ichki xato", "Internal error", nil)
	}

	_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
		OrderId: updated.OrderID,
		Status:  "WAITING_OPERATOR",
	})

	return structs.PaymePerformResult{
		Transaction: updated.PaycomTransactionID,
		State:       updated.State,
		PerformTime: updated.PerformTime.Int64,
	}, structs.RPCError{}
}

func (s *service) CancelTransaction(ctx context.Context, p structs.PaymeCancelParams) (structs.PaymeCancelResult, structs.RPCError) {
	tx, err := s.paymeRepo.GetByPaycomTransactionID(ctx, p.Id)
	if err != nil {
		return structs.PaymeCancelResult{}, rpcErr(
			-31003,
			"Транзакция не найдена",
			"Tranzaksiya topilmadi",
			"Transaction not found",
			"transaction")
	}

	if tx.State < 0 {
		ct := int64(0)
		if tx.CancelTime.Valid {
			ct = tx.CancelTime.Int64
		}
		return structs.PaymeCancelResult{
			Transaction: tx.PaycomTransactionID,
			State:       tx.State,
			CancelTime:  ct,
		}, structs.RPCError{}
	}

	if tx.State != paymerepo.StateCreated && tx.State != paymerepo.StatePerformed {
		return structs.PaymeCancelResult{}, rpcErr(
			-32400,
			"Неверное состояние",
			"Noto‘g‘ri holat",
			"Invalid transaction state",
			"state")
	}

	newState := paymerepo.StateCanceledCreated
	shouldCancelOrder := false

	if tx.State == paymerepo.StatePerformed {
		newState = paymerepo.StateCanceledPerformed
		shouldCancelOrder = true
	}

	cancelAt := nowMs()

	updated, err := s.paymeRepo.MarkCanceled(ctx, p.Id, cancelAt, p.Reason, newState)
	if err != nil {
		s.logger.Error(ctx, "payme MarkCanceled failed", zap.Error(err))
		return structs.PaymeCancelResult{}, rpcErr(
			-32400,
			"Внутренняя ошибка",
			"Ichki xato",
			"Internal error",
			nil)
	}

	if shouldCancelOrder {
		_ = s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: updated.OrderID,
			Status:  "CANCELLED",
		})
	}

	ct := cancelAt
	if updated.CancelTime.Valid {
		ct = updated.CancelTime.Int64
	}

	return structs.PaymeCancelResult{
		Transaction: updated.PaycomTransactionID,
		State:       updated.State,
		CancelTime:  ct,
	}, structs.RPCError{}
}

func (s *service) CheckTransaction(ctx context.Context, p structs.PaymeCheckParams) (structs.PaymeCheckResult, structs.RPCError) {
	tx, err := s.paymeRepo.GetByPaycomTransactionID(ctx, p.Id)
	if err != nil {
		return structs.PaymeCheckResult{}, rpcErr(-31003, "Транзакция не найдена", "Tranzaksiya topilmadi", "Transaction not found", "transaction")
	}

	// Spec uchun deterministik (har call’da bir xil):
	performTime := int64(0)
	cancelTime := int64(0)

	if tx.PerformTime.Valid {
		performTime = tx.PerformTime.Int64
	}
	if tx.CancelTime.Valid {
		cancelTime = tx.CancelTime.Int64
	}

	var reasonPtr *int
	// reason faqat canceled holatda bo‘lsin, boshqa holatda null
	if tx.State < 0 && tx.Reason.Valid {
		r := int(tx.Reason.Int64)
		reasonPtr = &r
	}

	return structs.PaymeCheckResult{
		Transaction: tx.PaycomTransactionID,
		State:       tx.State,
		CreateTime:  tx.CreatedTime,
		PerformTime: performTime,
		CancelTime:  cancelTime,
		Reason:      reasonPtr,
	}, structs.RPCError{}
}

func (s *service) GetStatement(ctx context.Context, p structs.PaymeStatementParams) (structs.PaymeStatementResult, structs.RPCError) {
	txs, err := s.paymeRepo.GetStatement(ctx, p.From, p.To)
	if err != nil {
		s.logger.Error(ctx, "payme GetStatement failed", zap.Error(err))
		return structs.PaymeStatementResult{}, rpcErr(
			-32400,
			"Внутренняя ошибка",
			"Ichki xato",
			"Internal error",
			nil)
	}

	out := make([]structs.Transaction, 0, len(txs))
	for _, tx := range txs {
		amountTiyin, e := somStringToTiyin(tx.Amount)
		if e != nil {
			return structs.PaymeStatementResult{}, rpcErr(
				-32400,
				"Внутренняя ошибка",
				"Ichki xato",
				"Internal error",
				nil)
		}

		var reasonPtr *int
		if tx.State < 0 && tx.Reason.Valid {
			r := int(tx.Reason.Int64)
			reasonPtr = &r
		}

		item := structs.Transaction{
			Id:         tx.PaycomTransactionID,
			Time:       tx.CreatedTime,
			Amount:     amountTiyin,
			Account:    structs.Account{OrderID: tx.OrderID},
			CreateTime: tx.CreatedTime,
			State:      tx.State,
			Reason:     reasonPtr,
		}
		if tx.PerformTime.Valid {
			item.PerformTime = tx.PerformTime.Int64
		}
		if tx.CancelTime.Valid {
			item.CancelTime = tx.CancelTime.Int64
		}

		out = append(out, item)
	}

	return structs.PaymeStatementResult{Transactions: out}, structs.RPCError{}
}

func (s *service) BuildPaymeCheckoutURL(merchantID string, orderID string, amountTiyin int64) (string, error) {
	if merchantID == "" {
		return "", fmt.Errorf("PAYME_MERCHANT_ID is empty")
	}
	if orderID == "" {
		return "", fmt.Errorf("orderID is empty")
	}
	if amountTiyin <= 0 {
		return "", fmt.Errorf("amountTiyin must be > 0")
	}

	// URL-safe base64 (padding siz), path’da muammo bo‘lmasin
	params := fmt.Sprintf("m=%s;ac.order_id=%s;a=%d", merchantID, orderID, amountTiyin)
	enc := base64.RawURLEncoding.EncodeToString([]byte(params))
	return "https://checkout.paycom.uz/" + enc, nil
}
