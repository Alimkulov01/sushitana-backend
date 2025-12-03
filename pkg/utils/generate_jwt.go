package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

func GenerateJWT(adminID string, roleID string) (string, error) {
	secret := os.Getenv("SECRET_KEY")
	if secret == "" {
		return "", fmt.Errorf("SECRET_KEY is not set")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":      adminID,
		"role_id": roleID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ParseJWT(tokenString string) (jwt.MapClaims, error) {
	secret := os.Getenv("SECRET_KEY")
	if secret == "" {
		return nil, fmt.Errorf("SECRET_KEY is not set")
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if token == nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	if err := claims.Valid(); err != nil {
		return nil, fmt.Errorf("token claims invalid: %w", err)
	}

	return claims, nil
}

func ParseJWTExp(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("not jwt")
	}
	payload := parts[1]
	// add padding
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		b, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return 0, err
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return 0, err
	}
	if expV, ok := obj["exp"]; ok {
		switch v := expV.(type) {
		case float64:
			return int64(v), nil
		case int64:
			return v, nil
		case json.Number:
			i, _ := v.Int64()
			return i, nil
		default:
			return 0, fmt.Errorf("unknown exp type %T", v)
		}
	}
	return 0, fmt.Errorf("exp not found")
}
