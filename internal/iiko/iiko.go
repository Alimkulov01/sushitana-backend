package iiko

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	iikorepo "sushitana/pkg/repository/postgres/iiko_repo"
	"sushitana/pkg/utils"
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
		Logger   logger.Logger
		IIKORepo iikorepo.Repo
	}

	Service interface {
		GetIikoAccessToken(ctx context.Context) (resp structs.IikoClientTokenResponse, err error)
		CreateOrder(ctx context.Context, req structs.IikoCreateDeliveryRequest) (structs.IikoCreateDeliveryResponse, error)
		GetCategory(ctx context.Context, token string, req structs.GetCategoryMenuRequest) (structs.GetCategoryResponse, error)
		GetProduct(ctx context.Context, token string, req structs.GetCategoryMenuRequest) (structs.GetProductResponse, error)
		UpdateIIKO(ctx context.Context, id int64, token string) (int64, error)
		CreatePickup(ctx context.Context, req structs.IikoCreateDeliveryRequest) (structs.IikoCreateDeliveryResponse, error) // NEW
		EnsureValidIikoToken(ctx context.Context) (string, error)
	}

	service struct {
		logger   logger.Logger
		iikorepo iikorepo.Repo
	}
)

func New(p Params) Service {
	return &service{
		logger:   p.Logger,
		iikorepo: p.IIKORepo,
	}
}

func (s *service) GetIikoAccessToken(ctx context.Context) (structs.IikoClientTokenResponse, error) {
	var resp structs.IikoClientTokenResponse

	apiLogin := os.Getenv("IIKO_API_LOGIN")
	if apiLogin == "" {
		return resp, fmt.Errorf("IIKO_API_LOGIN empty")
	}

	baseUrl := "https://api-ru.iiko.services/api/1/access_token"
	jsonData, _ := json.Marshal(structs.IikoClientTokenRequest{ApiLogin: apiLogin})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseUrl, bytes.NewReader(jsonData))
	if err != nil {
		return resp, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		s.logger.Error(ctx, "token endpoint returned non-2xx", zap.Int("status", httpResp.StatusCode), zap.ByteString("body", body))
		return resp, fmt.Errorf("token endpoint returned %d: %s", httpResp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		s.logger.Error(ctx, "failed to unmarshal token response", zap.Error(err), zap.ByteString("body", body))
		return resp, err
	}

	if err := s.iikorepo.CreateIIKO(ctx, resp.Token); err != nil {
		s.logger.Error(ctx, "failed to upsert iiko token", zap.Error(err))
		return resp, fmt.Errorf("upsert token failed: %w", err)
	}

	s.logger.Info(ctx, "obtained and upserted new IIKO token", zap.Int("token_len", len(resp.Token)))
	return resp, nil
}

func (s *service) GetCategory(ctx context.Context, token string, req structs.GetCategoryMenuRequest) (structs.GetCategoryResponse, error) {
	var result structs.GetCategoryResponse

	t := token
	if t == "" {
		var err error
		t, err = s.EnsureValidIikoToken(ctx)
		if err != nil {
			return result, fmt.Errorf("failed get token: %w", err)
		}
	}

	do := func(tok string) (int, []byte, error) {
		baseUrl := "https://api-ru.iiko.services/api/1/nomenclature"
		b := utils.Marshal(req)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseUrl, bytes.NewReader(b))
		if err != nil {
			return 0, nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		if tok != "" {
			httpReq.Header.Set("Authorization", "Bearer "+tok)
		}
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, body, nil
	}

	status, body, err := do(t)
	if err != nil {
		return result, err
	}

	if status == http.StatusUnauthorized {
		s.logger.Info(ctx, "GetCategory received 401; fetching new token and retrying")
		tr, err := s.GetIikoAccessToken(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to refresh token after 401: %w", err)
		}
		status, body, err = do(tr.Token)
		if err != nil {
			return result, err
		}
		if status == http.StatusUnauthorized {
			return result, fmt.Errorf("unauthorized even after token refresh")
		}
	}

	if status < 200 || status >= 300 {
		return result, fmt.Errorf("iiko returned status %d: %s", status, string(body))
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		s.logger.Error(ctx, "Failed to unmarshal iiko response", zap.Error(err), zap.ByteString("body", body))
		return result, err
	}

	if corr, ok := raw["correlationId"].(string); ok {
		result.CorrelationId = corr
	}
	if groupsAny, ok := raw["groups"]; ok {
		if groupsSlice, ok := groupsAny.([]any); ok {
			for _, gi := range groupsSlice {
				gm, ok := gi.(map[string]any)
				if !ok {
					s.logger.Warn(ctx, "group item is not object", zap.Any("item", gi))
					continue
				}
				g := structs.IikoGroup{
					Id:               safeStr(gm["id"]),
					ParentGroup:      safeStr(gm["parentGroup"]),
					IsIncludedInMenu: safeBool(gm["isIncludedInMenu"]),
					IsGroupModifier:  safeBool(gm["isGroupModifier"]),
					Name:             safeStr(gm["name"]),
					IsDeleted:        safeBool(gm["isDeleted"]),
				}
				if g.Id == "" {
					s.logger.Warn(ctx, "group without id, skipping", zap.Any("raw", gm))
					continue
				}
				result.Groups = append(result.Groups, g)
			}
		}
	}

	return result, nil
}

func (s *service) GetProduct(ctx context.Context, token string, req structs.GetCategoryMenuRequest) (structs.GetProductResponse, error) {
	var result structs.GetProductResponse

	t := token
	if t == "" {
		var err error
		t, err = s.EnsureValidIikoToken(ctx)
		if err != nil {
			return result, fmt.Errorf("failed get token: %w", err)
		}
	}

	do := func(tok string) (int, []byte, error) {
		baseUrl := "https://api-ru.iiko.services/api/1/nomenclature"
		b := utils.Marshal(req)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseUrl, bytes.NewReader(b))
		if err != nil {
			return 0, nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		if tok != "" {
			httpReq.Header.Set("Authorization", "Bearer "+tok)
		}
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, body, nil
	}

	status, body, err := do(t)
	if err != nil {
		return result, err
	}

	if status == http.StatusUnauthorized {
		s.logger.Info(ctx, "GetProduct received 401; fetching new token and retrying")
		tr, err := s.GetIikoAccessToken(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to refresh token after 401: %w", err)
		}
		status, body, err = do(tr.Token)
		if err != nil {
			return result, err
		}
		if status == http.StatusUnauthorized {
			return result, fmt.Errorf("unauthorized even after token refresh")
		}
	}

	if status < 200 || status >= 300 {
		return result, fmt.Errorf("iiko returned status %d: %s", status, string(body))
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		s.logger.Error(ctx, "Failed to unmarshal iiko response", zap.Error(err), zap.ByteString("body", body))
		return result, err
	}

	if corr, ok := raw["correlationId"].(string); ok {
		result.CorrelationId = corr
	}
	if productsAny, ok := raw["products"]; ok {
		if productsSlice, ok := productsAny.([]any); ok {
			for _, pi := range productsSlice {
				pm, ok := pi.(map[string]any)
				if !ok {
					s.logger.Warn(ctx, "product item is not object", zap.Any("item", pi))
					continue
				}
				var p structs.IIKOProduct
				p.ID = safeStr(pm["id"])
				p.Name = safeStr(pm["name"])
				p.GroupID = safeStr(pm["groupId"])
				if pm["productCategoryId"] == nil {
					p.ProductCategoryID = ""
				} else {
					p.ProductCategoryID = safeStr(pm["productCategoryId"])
				}
				p.Type = safeStr(pm["type"])
				p.OrderItemType = safeStr(pm["orderItemType"])
				p.MeasureUnit = safeStr(pm["measureUnit"])
				p.DoNotPrintInCheque = safeBool(pm["doNotPrintInCheque"])
				p.ParentGroup = safeStr(pm["parentGroup"])
				p.Order = safeInt(pm["order"])
				p.PaymentSubject = safeStr(pm["paymentSubject"])
				p.Code = safeStr(pm["code"])
				p.IsDeleted = safeBool(pm["isDeleted"])
				p.CanSetOpenPrice = safeBool(pm["canSetOpenPrice"])
				p.Splittable = safeBool(pm["splittable"])
				p.Weight = safeFloat(pm["weight"])

				if spAny, ok := pm["sizePrices"]; ok {
					if spSlice, ok := spAny.([]any); ok && len(spSlice) > 0 {
						for _, sitem := range spSlice {
							if sm, ok := sitem.(map[string]any); ok {
								b, _ := json.Marshal(sm)
								var sp structs.SizePrice
								if err := json.Unmarshal(b, &sp); err == nil {
									p.SizePrices = append(p.SizePrices, sp)
								} else {
									s.logger.Warn(ctx, "Failed to unmarshal sizePrice", zap.Error(err))
								}
							}
						}
					}
				}
				if p.ID == "" {
					s.logger.Warn(ctx, "product without id, skipping", zap.Any("raw", pm))
					continue
				}

				result.Products = append(result.Products, p)
			}
		}
	}

	return result, nil
}

func (s service) UpdateIIKO(ctx context.Context, id int64, token string) (int64, error) {
	rowsAffected, err := s.iikorepo.UpdateIIKO(ctx, id, token)
	if err != nil {
		s.logger.Error(ctx, "->iikorepo.Patch", zap.Error(err))
		return rowsAffected, err
	}
	return rowsAffected, err
}

func safeStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.(json.RawMessage); ok {
		var out string
		if err := json.Unmarshal(b, &out); err == nil {
			return out
		}
	}
	return ""
}

func safeFloat(v any) float64 {
	if v == nil {
		return 0.0
	}
	if s, ok := v.(float64); ok {
		return s
	}
	if b, ok := v.(json.RawMessage); ok {
		var out float64
		if err := json.Unmarshal(b, &out); err == nil {
			return out
		}
	}
	return 0.0
}

func safeInt(v any) int64 {
	if v == nil {
		return 0
	}
	if s, ok := v.(int64); ok {
		return s
	}
	if b, ok := v.(json.RawMessage); ok {
		var out int64
		if err := json.Unmarshal(b, &out); err == nil {
			return out
		}
	}
	return 0
}

func safeBool(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	switch t := v.(type) {
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	}
	return false
}

func (s *service) EnsureValidIikoToken(ctx context.Context) (string, error) {
	token, err := s.iikorepo.GetTokenIIKO(ctx, 1)
	if err != nil {
		if err == sql.ErrNoRows {
			tr, err := s.GetIikoAccessToken(ctx)
			if err != nil {
				return "", err
			}
			return tr.Token, nil
		}
		s.logger.Error(ctx, "failed to read stored token", zap.Error(err))
		return "", err
	}
	return token, nil
}

// CreateOrder sends order to iiko (/api/1/deliveries/create).
// NOTE: In your setup, PICKUP is "DeliveryPickUp" orderServiceType and can be sent here too.
// We require DeliveryPoint ONLY for courier delivery orderType.
func (s *service) CreateOrder(ctx context.Context, req structs.IikoCreateDeliveryRequest) (structs.IikoCreateDeliveryResponse, error) {
	var result structs.IikoCreateDeliveryResponse
	start := time.Now()

	token, err := s.EnsureValidIikoToken(ctx)
	if err != nil {
		s.logger.Error(ctx, "IIKO EnsureValidIikoToken failed", zap.Error(err))
		return result, fmt.Errorf("EnsureValidIikoToken: %w", err)
	}

	// Validate + normalize request
	if err := validateAndNormalizeIikoDeliveryCreate(&req); err != nil {
		return result, err
	}

	const baseURL = "https://api-ru.iiko.services/api/1/deliveries/create"

	do := func(attempt, tok string) (int, []byte, error) {
		payload, err := json.Marshal(req)
		if err != nil {
			return 0, nil, err
		}

		if s.logger != nil {
			pp, _ := json.MarshalIndent(req, "", "  ")
			s.logger.Info(ctx, "IIKO create request payload",
				zap.String("attempt", attempt),
				zap.Int("payload_len", len(payload)),
				zap.ByteString("payload", pp),
			)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(payload))
		if err != nil {
			return 0, nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+tok)

		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		// correlationId log even on non-2xx
		var tmp struct {
			CorrelationId string `json:"correlationId"`
		}
		_ = json.Unmarshal(body, &tmp)

		s.logger.Info(ctx, "IIKO create attempt finished",
			zap.String("attempt", attempt),
			zap.Int("status", resp.StatusCode),
			zap.Duration("duration", time.Since(start)),
			zap.Int("resp_body_len", len(body)),
			zap.String("correlationId", tmp.CorrelationId),
			zap.ByteString("resp_body", body),
		)

		return resp.StatusCode, body, nil
	}

	status, body, err := do("first", token)
	if err != nil {
		s.logger.Error(ctx, "IIKO create first attempt failed", zap.Error(err))
		return result, err
	}

	// Retry once on 401
	if status == http.StatusUnauthorized {
		s.logger.Info(ctx, "CreateOrder received 401; refreshing token and retrying")
		tr, err := s.GetIikoAccessToken(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to refresh token after 401: %w", err)
		}
		status, body, err = do("retry", tr.Token)
		if err != nil {
			return result, err
		}
		if status == http.StatusUnauthorized {
			return result, fmt.Errorf("unauthorized from iiko even after token refresh: %s", string(body))
		}
	}

	if status < 200 || status >= 300 {
		return result, fmt.Errorf("iiko deliveries/create returned %d: %s", status, string(body))
	}

	if err := json.Unmarshal(body, &result); err != nil {
		s.logger.Error(ctx, "CreateOrder: unmarshal response failed", zap.Error(err), zap.ByteString("body", body))
		return result, err
	}

	s.logger.Info(ctx, "IIKO create SUCCESS parsed",
		zap.String("externalNumber", result.OrderInfo.ExternalNumber),
		zap.String("iikoOrderId", result.OrderInfo.ID),
		zap.String("posId", result.OrderInfo.PosID),
		zap.String("creationStatus", result.OrderInfo.CreationStatus),
		zap.String("correlationId", result.CorrelationId),
	)

	return result, nil
}

func validateAndNormalizeIikoDeliveryCreate(req *structs.IikoCreateDeliveryRequest) error {
	if req.OrganizationId == "" || req.TerminalGroupId == "" {
		return fmt.Errorf("iiko request missing organizationId/terminalGroupId")
	}
	if req.Order.Items == nil {
		return fmt.Errorf("iiko request missing order")
	}

	req.Order.Phone = normalizePhone(req.Order.Phone)
	if req.Order.Phone == "" {
		return fmt.Errorf("iiko request missing order.phone")
	}
	if !strings.HasPrefix(req.Order.Phone, "+") {
		return fmt.Errorf("iiko order.phone must start with '+': %q", req.Order.Phone)
	}

	if req.Order.OrderTypeId == "" {
		return fmt.Errorf("iiko request missing order.orderTypeId")
	}

	if len(req.Order.Items) == 0 {
		return fmt.Errorf("iiko request has empty order.items")
	}
	for i := range req.Order.Items {
		if req.Order.Items[i].Type == "" {
			req.Order.Items[i].Type = "Product"
		}
	}

	// ✅ FIX: deliveryPoint/address faqat DELIVERY orderTypeId bo‘lsa majburiy
	if err := requireDeliveryAddressIfNeeded(req); err != nil {
		return err
	}

	// PaymentTypeKind normalize (Cash vs External)
	for i := range req.Order.Payments {
		p := &req.Order.Payments[i]
		switch strings.ToUpper(strings.TrimSpace(p.PaymentTypeKind)) {
		case "", "CASH":
			p.PaymentTypeKind = "Cash"
		case "ONLINE", "EXTERNAL":
			p.PaymentTypeKind = "External"
			if !p.IsProcessedExternally {
				p.IsProcessedExternally = true
			}
		default:
			// leave as-is
		}
	}

	return nil
}

// requireDeliveryAddressIfNeeded enforces DeliveryPoint only for courier DELIVERY order type.
// PICKUP order type is allowed with DeliveryPoint=nil.
func requireDeliveryAddressIfNeeded(req *structs.IikoCreateDeliveryRequest) error {
	deliveryTypeID := strings.TrimSpace(os.Getenv("IIKO_DELIVERY_ORDER_TYPE_ID"))
	pickupTypeID := strings.TrimSpace(os.Getenv("IIKO_PICKUP_ORDER_TYPE_ID"))

	ot := strings.TrimSpace(req.Order.OrderTypeId)

	// If env is not set, keep backward-compatible strict behavior:
	// (you can choose to return error instead, but this is safer for prod rollout)
	if deliveryTypeID == "" && pickupTypeID == "" {
		// old behavior
		return requireDeliveryAddressStrict(req)
	}

	// Courier delivery => strict
	if deliveryTypeID != "" && ot == deliveryTypeID {
		return requireDeliveryAddressStrict(req)
	}

	// Pickup => no requirement
	if pickupTypeID != "" && ot == pickupTypeID {
		return nil
	}

	// Unknown orderTypeId => do not require by default
	return nil
}

func requireDeliveryAddressStrict(req *structs.IikoCreateDeliveryRequest) error {
	dp := req.Order.DeliveryPoint
	if dp == nil {
		return fmt.Errorf("iiko delivery requires order.deliveryPoint (nil)")
	}
	if dp.Address == nil {
		return fmt.Errorf("iiko delivery requires order.deliveryPoint.address (nil)")
	}

	house := strings.TrimSpace(dp.Address.House)
	if house == "" {
		return fmt.Errorf("iiko delivery requires address.house")
	}

	if dp.Comment != "" {
		dp.Comment = strings.TrimSpace(dp.Comment)
	}
	if dp.Address.Flat != "" {
		dp.Address.Flat = strings.TrimSpace(dp.Address.Flat)
	}
	if dp.Address.Entrance != "" {
		dp.Address.Entrance = strings.TrimSpace(dp.Address.Entrance)
	}
	if dp.Address.Floor != "" {
		dp.Address.Floor = strings.TrimSpace(dp.Address.Floor)
	}
	if dp.Address.Doorphone != "" {
		dp.Address.Doorphone = strings.TrimSpace(dp.Address.Doorphone)
	}

	return nil
}

func normalizePhone(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", "\t", "").Replace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "+") {
		return s
	}
	if strings.HasPrefix(s, "998") {
		return "+" + s
	}
	if len(s) == 9 && strings.HasPrefix(s, "9") { // 9xxxxxxxx
		return "+998" + s
	}
	return s
}

// ---- Optional: CreatePickup remains unchanged (you can keep it, or stop using it in order service) ----

func (s *service) CreatePickup(ctx context.Context, req structs.IikoCreateDeliveryRequest) (structs.IikoCreateDeliveryResponse, error) {
	var result structs.IikoCreateDeliveryResponse

	// ✅ Pickup uchun deliveryPoint majburiy emas
	if err := validateAndNormalizeIikoPickupCreate(&req); err != nil {
		return result, err
	}

	token, err := s.EnsureValidIikoToken(ctx)
	if err != nil {
		return result, err
	}

	baseURL := strings.TrimSpace(os.Getenv("IIKO_API_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api-ru.iiko.services"
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/1/orders/create"

	jsonData, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return result, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 20 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return result, err
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	status := httpResp.StatusCode

	if status < 200 || status >= 300 {
		return result, fmt.Errorf("iiko orders/create returned %d: %s", status, string(body))
	}

	if err := json.Unmarshal(body, &result); err != nil {
		s.logger.Error(ctx, "CreatePickup: unmarshal response failed", zap.Error(err), zap.ByteString("body", body))
		return result, err
	}

	s.logger.Info(ctx, "IIKO pickup create SUCCESS parsed",
		zap.String("externalNumber", result.OrderInfo.ExternalNumber),
		zap.String("iikoOrderId", result.OrderInfo.ID),
		zap.String("posId", result.OrderInfo.PosID),
		zap.String("creationStatus", result.OrderInfo.CreationStatus),
		zap.String("correlationId", result.CorrelationId),
	)

	return result, nil
}

func validateAndNormalizeIikoPickupCreate(req *structs.IikoCreateDeliveryRequest) error {
	if req.OrganizationId == "" || req.TerminalGroupId == "" {
		return fmt.Errorf("iiko request missing organizationId/terminalGroupId")
	}
	if req.Order.Items == nil {
		return fmt.Errorf("iiko request missing order")
	}

	req.Order.Phone = normalizePhone(req.Order.Phone)
	if req.Order.Phone == "" {
		return fmt.Errorf("iiko request missing order.phone")
	}
	if !strings.HasPrefix(req.Order.Phone, "+") {
		return fmt.Errorf("iiko order.phone must start with '+': %q", req.Order.Phone)
	}

	if req.Order.OrderTypeId == "" {
		return fmt.Errorf("iiko request missing order.orderTypeId")
	}

	if len(req.Order.Items) == 0 {
		return fmt.Errorf("iiko request has empty order.items")
	}
	for i := range req.Order.Items {
		if req.Order.Items[i].Type == "" {
			req.Order.Items[i].Type = "Product"
		}
	}

	for i := range req.Order.Payments {
		p := &req.Order.Payments[i]
		switch strings.ToUpper(strings.TrimSpace(p.PaymentTypeKind)) {
		case "", "CASH":
			p.PaymentTypeKind = "Cash"
		case "ONLINE", "EXTERNAL":
			p.PaymentTypeKind = "External"
			if !p.IsProcessedExternally {
				p.IsProcessedExternally = true
			}
		default:
		}
	}

	return nil
}
