package handler

import (
	"context"
	"net/http"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/sink"
	"10000hk.com/vip_gift/internal/types"

	"github.com/gofiber/fiber/v2"
)

type OrderHandler struct {
	svc service.OrderService
	api types.OrderApi
}

// NewOrderHandler 构造函数
func NewOrderHandler(svc service.OrderService, api types.OrderApi) *OrderHandler {
	return &OrderHandler{svc: svc, api: api}
}

// RegisterRoutes 注册路由
func (h *OrderHandler) RegisterRoutes(r fiber.Router) {

	// 1) 创建订单: POST /orders
	r.Post("/orders/create", h.CreateOrder)

	// 2) 获取单条订单 (改为 POST /orders/one)
	r.Post("/orders/one", h.GetOneOrder)

	// 3) 分页查看订单列表: POST /orders/list
	r.Post("/orders/list", h.ListOrders)

	r.Post("/orders/query", h.QueryOrders)
}

// -------------------------------------------------------------------
// 1) CreateOrder
// Body: { "downstreamOrderId":"XXX", "dataJSON":"XXX" }
// -------------------------------------------------------------------
func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	var req sink.OrderCreateReq
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, http.StatusBadRequest, err.Error())
	}
	// 0)

	// 1) 把 req 转成内部的 OrderDTO

	dto, _ := h.svc.ToOrderDto(context.Background(), req)

	// 2) 调用 service.CreateOrder
	out, err := h.svc.CreateOrder(context.Background(), &dto)
	if err != nil {
		return ErrorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// 3) 组装响应
	resp := sink.OrderCreateResp{
		OrderId: out.OrderId,
		Status:  out.Status,
		Message: "订单创建成功",
	}
	return SuccessJSON(c, resp)
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
		Page               int64    `json:"page"`
		Size               int64    `json:"size"`
		OrderIds           []string `json:"orderIds,omitempty"`           // 可选
		DownstreamOrderIds []string `json:"downstreamOrderIds,omitempty"` // 可选
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

	items, total, err := h.svc.ListOrder(context.Background(), req.Page, req.Size, req.OrderIds, req.DownstreamOrderIds)
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

// QueryOrders 转发订单查询请求
func (h *OrderHandler) QueryOrders(c *fiber.Ctx) error {
	var req struct {
		OrderIds []string `json:"orderIds"`
	}
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, 400, err.Error())
	}

	if len(req.OrderIds) == 0 {
		return ErrorJSON(c, 400, "orderIds is required")
	}

	var orderResults []sink.OrderQueryResp

	orderResults, err := h.api.DoQueryOrder(context.Background(), req.OrderIds)
	if err != nil {
		return SuccessJSON(c, orderResults)
	}

	// 统一返回所有订单的查询结果
	return SuccessJSON(c, orderResults)
}
