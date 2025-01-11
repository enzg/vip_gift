package handler

import (
	"context"
	"net/http"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/sink"
	"10000hk.com/vip_gift/internal/types"
	"10000hk.com/vip_gift/pkg"

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

	// 1) 创建订单: POST /orders
	r.Post("/orders/create", h.CreateOrder)

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
	var req sink.OrderCreateReq
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, http.StatusBadRequest, err.Error())
	}

	// 1) 把 req 转成内部的 OrderDTO
	//    其中 phone/publicCode/otac 你若想存进 DB，可拼到 dataJSON 或另想办法
	//    这里演示“把 phone、publicCode、otac”拼到 DataJSON 的 JSON 里。
	extraMap := map[string]interface{}{
		"phone":      req.Phone,
		"publicCode": req.PublicCode,
		"otac":       req.Otac,
	}
	finalDataJSON := pkg.MergeJSON(req.DataJSON, extraMap)
	// mergeJSON 是我们要写的一个小工具函数，见下方示例

	dto := &types.OrderDTO{
		DownstreamOrderId: req.DownstreamOrderId,
		DataJSON:          finalDataJSON, // 合并好的JSON
		Status:            0,             // 初始状态
		Remark:            "",            // 备注可空
	}

	// 2) 调用 service.CreateOrder
	out, err := h.svc.CreateOrder(context.Background(), dto)
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
