// internal/handler/common_response.go
package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type ListRequest struct {
	Page int64 `json:"page"`
	Size int64 `json:"size"`
}
type BatchCategoryRequest struct {
	Prefix   string `json:"prefix"`
	Category string `json:"category"`
}
type SearchRequest struct {
	Keyword string `json:"keyword"` // 可以加 omitempty
	Page    int64  `json:"page"`
	Size    int64  `json:"size"`
}

// 定义统一的响应结构
type BaseResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 通用成功响应：默认 code=200
func SuccessJSON(c *fiber.Ctx, data interface{}) error {
	return c.JSON(BaseResponse{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}

// 通用错误响应：可带自定义 code
func ErrorJSON(c *fiber.Ctx, code int, msg string) error {
	// 也可将 Fiber Status Code 与自定义Code统一
	return c.Status(http.StatusOK).JSON(BaseResponse{
		Code:    code,
		Message: msg,
		Data:    nil,
	})
}
func Unauthorized(c *fiber.Ctx, msg string) error {
	// 这里 HTTP Status 用 401，JSON 里 code 也给 401
	return c.Status(http.StatusUnauthorized).JSON(BaseResponse{
		Code:    http.StatusUnauthorized, // 401
		Message: msg,
		Data:    nil,
	})
}
