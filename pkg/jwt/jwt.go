package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims JWT声明结构体
type Claims struct {
	UserID               uint   `json:"user_id"` // 用户ID
	Account              string `json:"account"` // 用户账号
	TokenType            string `json:"token_type"`
	SessionID            string `json:"session_id"` // 会话ID，用于多设备踢下线
	RefreshID            string `json:"refresh_id,omitempty"`
	jwt.RegisteredClaims        // JWT标准声明（过期时间、签发时间等）
}

func (c *Claims) GetUserID() uint {
	return c.UserID
}

func (c *Claims) GetAccount() string {
	return c.Account
}

// 负责生成、解析和刷新JWT token
type JWTManager struct {
	secretKey []byte // JWT签名密钥
}

// NewJWTManager 创建JWT管理器实例
// secretKey: JWT签名密钥，用于签名和验证token
func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secretKey),
	}
}

func GenerateTokenID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ParseToken 解析并验证 JWT token
func (j *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return j.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ----------JWT 生成token（底层）----------
// GenerateTokenWithSession 生成带 session_id 的 token，sessionID 为空时兼容旧版
func (j *JWTManager) GenerateTokenWithSession(userID uint, account, tokenType, sessionID string, expiresIn time.Duration) (string, error) {
	return j.GenerateTokenWithSessionAndRefreshID(userID, account, tokenType, sessionID, "", expiresIn)
}

// GenerateTokenWithSessionAndRefreshID 生成带 session_id 和 refresh_id 的 token。
// refresh_id 只用于 refresh token 的服务端轮换校验。
func (j *JWTManager) GenerateTokenWithSessionAndRefreshID(userID uint, account, tokenType, sessionID, refreshID string, expiresIn time.Duration) (string, error) {
	claims := Claims{
		UserID:    userID,
		Account:   account,
		TokenType: tokenType,
		SessionID: sessionID,
		RefreshID: refreshID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)), // 过期时间
			IssuedAt:  jwt.NewNumericDate(time.Now()),                // 签发时间
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

// RefreshAccessToken 使用refresh token刷新access token
func (j *JWTManager) RefreshAccessToken(refreshToken string, expiresIn time.Duration) (string, error) {
	claims, err := j.ParseToken(refreshToken)
	if err != nil {
		return "", err
	}
	if claims.TokenType != TokenTypeRefresh {
		return "", errors.New("invalid refresh token")
	}
	return j.GenerateTokenWithSession(claims.UserID, claims.Account, TokenTypeAccess, claims.SessionID, expiresIn)
}

// RotateRefreshToken 轮换refresh token（旧refresh token换新refresh token）
func (j *JWTManager) RotateRefreshToken(refreshToken string, expiresIn time.Duration) (string, error) {
	claims, err := j.ParseToken(refreshToken)
	if err != nil {
		return "", err
	}
	if claims.TokenType != TokenTypeRefresh {
		return "", errors.New("invalid refresh token")
	}
	return j.GenerateTokenWithSession(claims.UserID, claims.Account, TokenTypeRefresh, claims.SessionID, expiresIn)
}
