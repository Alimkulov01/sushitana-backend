package payme

import (
	"context"
	"encoding/base64"
	"fmt"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"
	paymerepo "sushitana/pkg/repository/postgres/payment_repo/payme_repo"
	"time"

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
	var som int64
	var tiyin int64
	_, err := fmt.Sscanf(s, "%d.%d", &som, &tiyin)
	if err != nil {
		return 0, err
	}
	if tiyin < 0 || tiyin > 99 {
		return 0, fmt.Errorf("invalid tiyin")
	}
	return som*100 + tiyin, nil
}

func parseOrderID(p structs.Account) (string, bool) {
	if p.OrderID != "" {
		return p.OrderID, true
	}
	if p.ID != "" {
		return p.ID, true
	}
	return "", false
}

func (s *service) CheckPerformTransaction(ctx context.Context, p structs.PaymeCheckPerformParams) (structs.PaymeCheckPerformResult, structs.RPCError) {
	orderID, ok := parseOrderID(p.Account)
	if !ok {
		return structs.PaymeCheckPerformResult{}, structs.RPCError{
			Code: -32600, Message: "Invalid account", Data: "order_id required",
		}
	}

	ord, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return structs.PaymeCheckPerformResult{
				Allow: false,
			}, structs.RPCError{
				Code: -31050, Message: "Order not found", Data: orderID,
			}
	}

	if ord.Order.Status != "WAITING_PAYMENT" {
		return structs.PaymeCheckPerformResult{Allow: false}, structs.RPCError{
			Code: -31050, Message: "Order is not payable", Data: ord.Order.Status,
		}
	}

	expected := int64(ord.Order.TotalPrice * 100)
	if p.Amount != expected {
		return structs.PaymeCheckPerformResult{Allow: false}, structs.RPCError{
			Code: -31001, Message: "Incorrect amount", Data: fmt.Sprintf("expected=%d got=%d", expected, p.Amount),
		}
	}

	return structs.PaymeCheckPerformResult{Allow: true}, structs.RPCError{}
}

func (s *service) CreateTransaction(ctx context.Context, p structs.PaymeCreateParams) (structs.PaymeCreateResult, structs.RPCError) {
	orderID, ok := parseOrderID(p.Account)
	if !ok {
		return structs.PaymeCreateResult{}, structs.RPCError{Code: -32600, Message: "Invalid account", Data: "order_id required"}
	}

	ord, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return structs.PaymeCreateResult{}, structs.RPCError{Code: -31050, Message: "Order not found", Data: orderID}
	}

	// amount tekshiruv
	expected := int64(ord.Order.TotalPrice * 100)
	if p.Amount != expected {
		return structs.PaymeCreateResult{}, structs.RPCError{
			Code: -31001, Message: "Incorrect amount", Data: fmt.Sprintf("expected=%d got=%d", expected, p.Amount),
		}
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

	tx, err := s.paymeRepo.Create(ctx, orderID, p.Id, amountSom, p.Time)
	if err != nil {
		s.logger.Error(ctx, "payme CreateTransaction repo.Create failed", zap.Error(err))
		return structs.PaymeCreateResult{}, structs.RPCError{Code: -32400, Message: "Internal error"}
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
		return structs.PaymePerformResult{}, structs.RPCError{Code: -31003, Message: "Transaction not found", Data: p.Id}
	}

	if tx.State == paymerepo.StatePerformed {
		return structs.PaymePerformResult{
			Transaction: tx.PaycomTransactionID,
			State:       tx.State,
			PerformTime: tx.PerformTime.Int64,
		}, structs.RPCError{}
	}

	if tx.State < 0 {
		return structs.PaymePerformResult{}, structs.RPCError{Code: -31008, Message: "Transaction canceled", Data: tx.State}
	}

	updated, err := s.paymeRepo.MarkPerformed(ctx, p.Id, nowMs())
	if err != nil {
		s.logger.Error(ctx, "payme MarkPerformed failed", zap.Error(err))
		return structs.PaymePerformResult{}, structs.RPCError{Code: -32400, Message: "Internal error"}
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
		return structs.PaymeCancelResult{}, structs.RPCError{
			Code:    -31003,
			Message: "Transaction not found",
			Data:    p.Id,
		}
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
		return structs.PaymeCancelResult{}, structs.RPCError{
			Code:    -32400,
			Message: "Invalid transaction state",
			Data:    tx.State,
		}
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
		return structs.PaymeCancelResult{}, structs.RPCError{
			Code:    -32400,
			Message: "Internal error",
		}
	}

	if shouldCancelOrder {
		if err := s.orderRepo.UpdateStatus(ctx, structs.UpdateStatus{
			OrderId: updated.OrderID,
			Status:  "CANCELLED",
		}); err != nil {
			s.logger.Error(ctx, "orderRepo.UpdateStatus CANCELLED failed", zap.Error(err), zap.String("order_id", updated.OrderID))
		}
	}

	ct := int64(0)
	if updated.CancelTime.Valid {
		ct = updated.CancelTime.Int64
	} else {
		ct = cancelAt
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
		return structs.PaymeCheckResult{}, structs.RPCError{Code: -31003, Message: "Transaction not found", Data: p.Id}
	}

	res := structs.PaymeCheckResult{
		Transaction: tx.PaycomTransactionID,
		State:       tx.State,
		CreateTime:  tx.CreatedTime,
	}

	if tx.PerformTime.Valid {
		res.PerformTime = tx.PerformTime.Int64
	}
	if tx.CancelTime.Valid {
		res.CancelTime = tx.CancelTime.Int64
	}
	if tx.Reason.Valid {
		res.Reason = int(tx.Reason.Int64)
	}

	return res, structs.RPCError{}
}

func (s *service) GetStatement(ctx context.Context, p structs.PaymeStatementParams) (structs.PaymeStatementResult, structs.RPCError) {
	txs, err := s.paymeRepo.GetStatement(ctx, p.From, p.To)
	if err != nil {
		s.logger.Error(ctx, "payme GetStatement failed", zap.Error(err))
		return structs.PaymeStatementResult{}, structs.RPCError{Code: -32400, Message: "Internal error"}
	}

	out := make([]structs.Transaction, 0, len(txs))
	for _, tx := range txs {
		amountTiyin, e := somStringToTiyin(tx.Amount)
		if e != nil {
			return structs.PaymeStatementResult{}, structs.RPCError{Code: -32400, Message: "Internal error"}
		}

		item := structs.Transaction{
			Id:         tx.PaycomTransactionID,
			Time:       tx.CreatedTime,
			Amount:     amountTiyin,
			Account:    structs.Account{OrderID: tx.OrderID},
			CreateTime: tx.CreatedTime,
			State:      tx.State,
		}
		if tx.PerformTime.Valid {
			item.PerformTime = tx.PerformTime.Int64
		}
		if tx.CancelTime.Valid {
			item.CancelTime = tx.CancelTime.Int64
		}
		if tx.Reason.Valid {
			item.Reason = int(tx.Reason.Int64)
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

	// Payme format: base64("m=<merchant_id>;ac.order_id=<order_id>;a=<amount_tiyin>")
	params := fmt.Sprintf("m=%s;ac.order_id=%s;a=%d", merchantID, orderID, amountTiyin)

	enc := base64.StdEncoding.EncodeToString([]byte(params))
	return "https://checkout.paycom.uz/" + enc, nil
}
