package iiko

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
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
		Logger logger.Logger
	}

	Service interface {
		GetIikoAccessToken(ctx context.Context, req structs.IikoClientTokenRequest) (resp structs.IikoClientTokenResponse, err error)
		GetOrganization(ctx context.Context, token string) (resp structs.GetOrganizationResponse, err error)
		GetCategoryMenu(ctx context.Context, token string, req structs.GetCategoryMenuRequest) (structs.GetCategoryMenuResponse, error)
	}
	service struct {
		logger logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		logger: p.Logger,
	}
}

func (s service) GetIikoAccessToken(ctx context.Context, req structs.IikoClientTokenRequest) (resp structs.IikoClientTokenResponse, err error) {
	baseUrl := "https://api-ru.iiko.services/api/1/access_token"
	jsonData := utils.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseUrl, bytes.NewReader(jsonData))
	if err != nil {
		s.logger.Error(ctx, "Failed to create HTTP request: %v", zap.Error(err))
		return structs.IikoClientTokenResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "HTTP request failed: %v", zap.Error(err))
		return structs.IikoClientTokenResponse{}, err
	}
	defer httpResp.Body.Close()

	var result structs.IikoClientTokenResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		s.logger.Error(ctx, "Failed to decode response: %v", zap.Error(err))
		return structs.IikoClientTokenResponse{}, err
	}

	return result, nil
}

func (s service) GetOrganization(ctx context.Context, token string) (structs.GetOrganizationResponse, error) {
	fmt.Println(token)
	url := "https://api-ru.iiko.services/api/1/organizations"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		s.logger.Error(ctx, "failed to create request", zap.Error(err))
		return structs.GetOrganizationResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	httpResp, err := client.Do(req)
	if err != nil {
		s.logger.Error(ctx, "http request failed", zap.Error(err))
		return structs.GetOrganizationResponse{}, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(httpResp.Body)
		s.logger.Error(ctx, "non-2xx response", zap.Int("status", httpResp.StatusCode), zap.ByteString("body", body))
		return structs.GetOrganizationResponse{}, fmt.Errorf("unexpected status %d: %s", httpResp.StatusCode, string(body))
	}

	var result structs.GetOrganizationResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		body, _ := io.ReadAll(httpResp.Body)
		s.logger.Error(ctx, "failed to decode response", zap.Error(err), zap.ByteString("body", body))
		return structs.GetOrganizationResponse{}, err
	}

	return result, nil
}

func (s service) GetCategoryMenu(ctx context.Context, token string, req structs.GetCategoryMenuRequest) (structs.GetCategoryMenuResponse, error) {
	fmt.Println(token, req)
	result := structs.GetCategoryMenuResponse{}
	baseUrl := "https://api-ru.iiko.services/api/1/nomenclature"
	jsonData := utils.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseUrl, bytes.NewReader(jsonData))
	if err != nil {
		s.logger.Error(ctx, "Failed to create HTTP request: %v", zap.Error(err))
		return structs.GetCategoryMenuResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", token)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		s.logger.Error(ctx, "HTTP request failed: %v", zap.Error(err))
		return structs.GetCategoryMenuResponse{}, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(httpResp.Body)
		s.logger.Error(ctx, "non-2xx response", zap.Int("status", httpResp.StatusCode), zap.ByteString("body", body))
		return structs.GetCategoryMenuResponse{}, fmt.Errorf("unexpected status %d: %s", httpResp.StatusCode, string(body))
	}

	var raw map[string]any
	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &raw); err != nil {
		s.logger.Error(ctx, "Failed to unmarshal", zap.Error(err), zap.ByteString("body", body))
		return structs.GetCategoryMenuResponse{}, err
	}
	result.CorrelationId = raw["correlationId"].(string)
	groups := raw["groups"].([]any)
	for _, g := range groups {
		m := g.(map[string]any)
		result.Groups = append(result.Groups, structs.IikoGroup{
			Id:               m["id"].(string),
			ParentGroup:      m["parentGroup"].(string),
			IsIncludedInMenu: m["isIncludedInMenu"].(bool),
			IsGroupModifier:  m["isGroupModifier"].(bool),
			Name:             m["name"].(string),
			IsDeleted:        m["isDeleted"].(bool),
		})
	}
	products := raw["products"].([]any)
	for _, p := range products {
		m := p.(map[string]any)
		result.Products = append(result.Products, structs.IikoProduct{
			Id:             m["id"].(string),
			GroupId:        m["groupId"].(string),
			Weight:         m["weight"].(float64),
			Type:           m["type"].(string),
			OrderItemType:  m["orderItemType"].(string),
			MeasureUnit:    m["measureUnit"].(string),
			ParentGroup:    m["parentGroup"].(string),
			PaymentSubject: m["paymentSubject"].(string),
			Code:           m["code"].(string),
			Name:           m["name"].(string),
		})
	}

	return result, nil
}
