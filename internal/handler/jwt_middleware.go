// internal/handler/jwt_middleware.go
package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v4"
)

// CustomClaims 自定义的 JWT Claims 结构
type CustomClaims struct {
	jwt.RegisteredClaims
	UserSn  string `json:"userSn"`
	JwtHash string `json:"jwtHash"`
}

// JWTMiddleware 返回一个 Fiber handler，用于验证 JWT
func JWTMiddleware(secret string) fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey:     []byte(secret), // 用于签名验证
		TokenLookup:    "header:Authorization",
		AuthScheme:     "Bearer",
		SigningMethod:  "HS256",         // 如果你的 JWT 用的是 HS256
		ErrorHandler:   jwtError,        // 自定义错误处理
		SuccessHandler: jwtSuccess,      // 成功解析后的逻辑
		Claims:         &CustomClaims{}, // 使用自定义 Claims
	})
}

// jwtError 处理 JWT 验证失败的情况
func jwtError(c *fiber.Ctx, err error) error {
	// 这里可以根据 err 类型做更细致的处理，如 token 过期、签名错误等
	return Unauthorized(c, err.Error())
}

// jwtSuccess 处理 JWT 验证成功
func jwtSuccess(c *fiber.Ctx) error {
	// Fiber-JWT 会将解析后的 token 存到 c.Locals("user") 中
	token := c.Locals("user").(*jwt.Token)
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return Unauthorized(c, "Invalid token claims")
	}

	// 再次检查过期时间
	if !claims.ExpiresAt.Time.After(time.Now()) {
		return Unauthorized(c, "Token expired")
	}

	// 检查必需字段
	if claims.UserSn == "" {
		return Unauthorized(c, "Missing userSn")
	}

	if claims.JwtHash == "" {
		return Unauthorized(c, "Missing jwtHash")
	}

	// 将自定义数据放到 Context，后续业务可在 c.Locals("userSn") 里取
	c.Locals("userSn", claims.UserSn)
	c.Locals("jwtHash", claims.JwtHash)

	// 继续下一步
	return c.Next()
}
