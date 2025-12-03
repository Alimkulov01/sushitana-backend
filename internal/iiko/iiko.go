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
		GetCategory(ctx context.Context, token string, req structs.GetCategoryMenuRequest) (structs.GetCategoryResponse, error)
		UpdateIIKO(ctx context.Context, id int64, token string) (int64, error)
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
