package handler

import (
	"context"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"

	"github.com/gofiber/fiber/v2"
)

// OrderHandler 处理订单相关接口
type OrderHandler struct {
	svc service.OrderService
}

// NewOrderHandler 构造函数
func NewOrderHandler(svc service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// RegisterRoutes 注册路由
func (h *OrderHandler) RegisterRoutes(r fiber.Router) {
	// 如果你有 JWT 校验，可在外部 r.Use(JWTMiddleware(...))
	r.Post("/orders", h.CreateOrder)      // POST /orders
	r.Get("/orders/:orderId", h.GetOrder) // GET  /orders/:orderId
}

// CreateOrder
// Body: { "downstreamOrderId":"XXX", "dataJSON":"XXX" }
func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	var dto types.OrderDTO
	if err := c.BodyParser(&dto); err != nil {
		return ErrorJSON(c, 400, err.Error()) // 使用统一错误返回
	}

	// 调用 service
	out, err := h.svc.CreateOrder(context.Background(), &dto)
	if err != nil {
		return ErrorJSON(c, 500, err.Error()) // 统一错误返回
	}

	// 成功
	return SuccessJSON(c, out) // 统一成功
}

// GetOrder
// GET /orders/:orderId
func (h *OrderHandler) GetOrder(c *fiber.Ctx) error {
	orderId := c.Params("orderId", "")
	if orderId == "" {
		return ErrorJSON(c, 400, "orderId is required")
	}

	out, err := h.svc.GetOrder(context.Background(), orderId)
	if err != nil {
		return ErrorJSON(c, 404, err.Error()) 
	}

	return SuccessJSON(c, out)
}