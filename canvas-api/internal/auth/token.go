package auth

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const maxTokenLength = 8192

var (
	// ErrInvalidToken 表示签名无效、算法不符或结构损坏。
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired 表示 token 已过期。
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenTooLarge 表示 token 超长，提前拒绝以降低 DoS 风险。
	ErrTokenTooLarge = errors.New("token too large")
)

// JWTClaims 必须与 sub2api 的 service.JWTClaims 字段完全对齐，
// 这样 canvas-api 才能验证 sub2api 用同一 secret（HS256）签发的 access token。
type JWTClaims struct {
	UserID       int64  `json:"user_id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	TokenVersion int64  `json:"token_version"`
	SessionID    string `json:"sid,omitempty"`
	BindingHash  string `json:"bnd,omitempty"`
	jwt.RegisteredClaims
}

// ParseToken 校验签名与标准声明（exp/nbf），只接受 HMAC 系列算法，
// 防止算法混淆攻击。过期时仍返回 claims 并附带 ErrTokenExpired。
func ParseToken(tokenString, secret string) (*JWTClaims, error) {
	if len(tokenString) > maxTokenLength {
		return nil, ErrTokenTooLarge
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{
		jwt.SigningMethodHS256.Name,
		jwt.SigningMethodHS384.Name,
		jwt.SigningMethodHS512.Name,
	}))

	token, err := parser.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			if claims, ok := token.Claims.(*JWTClaims); ok {
				return claims, ErrTokenExpired
			}
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrInvalidToken
}

// ComputeTokenVersion 复刻 sub2api 的 resolvedTokenVersion。
//
// sub2api 的 users 表没有 token_version 列，所以存储值恒为 0，
// 最终 token_version = 0 ^ fingerprint = fingerprint，其中
// fingerprint = int64(BigEndian(SHA256(lower(trim(email)) + "\n" + passwordHash)[:8]) & 0x7fff...)。
//
// 用户改密码 → password_hash 变 → 指纹变 → 旧 token 的 token_version 不再匹配 → 失效。
func ComputeTokenVersion(email, passwordHash string) int64 {
	material := strings.ToLower(strings.TrimSpace(email)) + "\n" + passwordHash
	sum := sha256.Sum256([]byte(material))
	return int64(binary.BigEndian.Uint64(sum[:8]) & 0x7fffffffffffffff)
}
