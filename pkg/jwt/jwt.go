package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ClaimsInterface interface {
	GetUserID() uint
	GetAccount() string
}

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

// ----------JWT 解析token----------
// 传入: ctx context.Context          (上下文控制对象)
// 传入: tokenString string           (待解析的token字符串)
// 返回: *Claims                      (解析后的Claims结构体) / 返回: error (错误信息，成功则为nil)
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
	claims := Claims{
		UserID:    userID,
		Account:   account,
		TokenType: tokenType,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)), // 过期时间
			IssuedAt:  jwt.NewNumericDate(time.Now()),                // 签发时间
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

func (j *JWTManager) GenerateTokenWithType(userID uint, account, tokenType string, expiresIn time.Duration) (string, error) {
	return j.GenerateTokenWithSession(userID, account, tokenType, "", expiresIn)
}

// ----------JWT 生成token（便捷方法）----------
// GenerateToken 生成Access Token
// 传入: userID uint                  (用户ID)
// 传入: account string               (用户账号)
// 传入: expiresIn time.Duration      (token有效期)
// 返回: string                       (生成的token字符串) / 返回: error (错误信息，成功则为nil)
func (j *JWTManager) GenerateToken(userID uint, account string, expiresIn time.Duration) (string, error) {
	return j.GenerateTokenWithType(userID, account, TokenTypeAccess, expiresIn)
}

// GenerateRefreshToken 生成Refresh Token
func (j *JWTManager) GenerateRefreshToken(userID uint, account string, expiresIn time.Duration) (string, error) {
	return j.GenerateTokenWithType(userID, account, TokenTypeRefresh, expiresIn)
}

// ----------JWT 刷新token----------
// RefreshToken 使用旧token刷新（兼容旧版，不区分token类型）
// 传入: tokenString string           (旧的token字符串)
// 传入: expiresIn time.Duration      (新token的有效期)
// 返回: string                       (新生成的token字符串) / 返回: error (错误信息，成功则为nil)
func (j *JWTManager) RefreshToken(tokenString string, expiresIn time.Duration) (string, error) {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return "", err
	}
	return j.GenerateToken(claims.UserID, claims.Account, expiresIn)
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
