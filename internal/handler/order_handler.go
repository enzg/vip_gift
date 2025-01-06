package handler

import (
	"context"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"

	"github.com/gofiber/fiber/v2"
)

type OrderHandler struct {
	svc service.OrderService
}

// NewOrderHandler 构造函数
func NewOrderHandler(svc service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// RegisterRoutes 注册路由
func (h *OrderHandler) RegisterRoutes(r fiber.Router) {
	// 需要 JWT 保护可在外层 r.Use(JWTMiddleware(...))

	// 1) 创建订单: POST /orders
	r.Post("/orders", h.CreateOrder)

	// 2) 获取单条订单 (改为 POST /orders/one)
	r.Post("/orders/one", h.GetOneOrder)

	// 3) 分页查看订单列表: POST /orders/list
	r.Post("/orders/list", h.ListOrders)
}

// -------------------------------------------------------------------
// 1) CreateOrder
// Body: { "downstreamOrderId":"XXX", "dataJSON":"XXX" }
// -------------------------------------------------------------------
func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	var dto types.OrderDTO
	if err := c.BodyParser(&dto); err != nil {
		return ErrorJSON(c, 400, err.Error())
	}

	out, err := h.svc.CreateOrder(context.Background(), &dto)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}
	return SuccessJSON(c, out)
}

// -------------------------------------------------------------------
// 2) GetOneOrder
// POST /orders/one
// Body: { "orderId":"xxxx" }
// -------------------------------------------------------------------
func (h *OrderHandler) GetOneOrder(c *fiber.Ctx) error {
	var req struct {
		OrderId string `json:"orderId"`
	}
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, 400, "invalid request body")
	}
	if req.OrderId == "" {
		return ErrorJSON(c, 400, "orderId is required")
	}

	out, err := h.svc.GetOrder(context.Background(), req.OrderId)
	if err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, out)
}

// -------------------------------------------------------------------
// 3) ListOrders
// POST /orders/list
// Body: { "page":1, "size":10 }
// -------------------------------------------------------------------
func (h *OrderHandler) ListOrders(c *fiber.Ctx) error {
	var req struct {
		Page int64 `json:"page"`
		Size int64 `json:"size"`
	}
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, 400, err.Error())
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 10
	}

	items, total, err := h.svc.ListOrder(context.Background(), req.Page, req.Size)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}

	// 返回 { "total":..., "dataList": [...] }
	resp := fiber.Map{
		"total":    total,
		"dataList": items,
	}
	return SuccessJSON(c, resp)
}
