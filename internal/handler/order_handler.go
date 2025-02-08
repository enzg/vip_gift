package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"10000hk.com/vip_gift/internal/proxy"
	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/sink"
	"10000hk.com/vip_gift/internal/types"

	"github.com/gofiber/fiber/v2"
)

type OrderHandler struct {
	svc service.OrderService
	pub service.PubService
}

// NewOrderHandler 构造函数
func NewOrderHandler(svc service.OrderService, pub service.PubService) *OrderHandler {
	return &OrderHandler{svc: svc, pub: pub}
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
	// 从 http headers 中获取用户信息
	userSn := c.Get("partnerId")
	if userSn == "" {
		return ErrorJSON(c, http.StatusUnauthorized, "partnerId is required")
	}
	parentSn := c.Get("parentSn")
	if parentSn == "" {
		return ErrorJSON(c, http.StatusUnauthorized, "parentSn is required")
	}
	req.PartnerId = userSn
	req.ParentSn = parentSn
	// 0)
	var api types.OrderApi
	switch {
	case strings.Contains(req.DownstreamOrderId, "VV"):
		api = proxy.NewGiftApi(map[string]string{}, h.pub)
	case strings.Contains(req.DownstreamOrderId, "VC"):
		api = proxy.NewChargeApi(map[string]string{})
	default:
		return ErrorJSON(c, http.StatusBadRequest, "downstreamOrderId is invalid")
	}

	// 1) 把 req 转成内部的 OrderDTO

	dto, _ := api.ToOrderDto(context.Background(), req)

	// 2) 调用 service.CreateOrder
	out, err := h.svc.CreateOrder(context.Background(), &dto)
	if err != nil {
		return ErrorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// 3) 组装响应
	resp := sink.OrderCreateResp{
		OrderId: out.OrderId,
		Status:  int64(out.Status),
		Message: out.Status.Remark(),
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
	// 将orderIds按照 VV或者 VC前缀分组
	// 1) 按前缀 VV 和 VC 分组
	var vvIds []string
	var vcIds []string

	for _, oid := range req.OrderIds {
		switch {
		case strings.HasPrefix(oid, "VV"):
			vvIds = append(vvIds, oid)
		case strings.HasPrefix(oid, "VC"):
			vcIds = append(vcIds, oid)
		}
	}
	// 2) 对两组分别发起查询 (示例) 并合并结果
	var orderResults []sink.OrderQueryResp

	giftApi := proxy.NewGiftApi(map[string]string{
		"QueryOrder": "https://api0.10000hk.com/api/product/gift/orders/query",
	}, h.pub)
	chargeApi := proxy.NewChargeApi(map[string]string{
		"QueryOrder": "https://gift.10000hk.com/api/charge/order/query",
	})

	// 2.1) 查询 VV 前缀订单
	if len(vvIds) > 0 {
		respVV, err := giftApi.DoQueryOrder(context.Background(), vvIds)
		if err != nil {
			// 如果其中一组查询失败，是否要直接返回？还是只返回成功的？
			// 根据实际需求决定，这里先示例继续处理
			// return ErrorJSON(c, 500, err.Error())
			fmt.Println("VV group query error:", err)
		}
		orderResults = append(orderResults, respVV...)
	}

	// 2.2) 查询 VC 前缀订单
	if len(vcIds) > 0 {
		respVC, err := chargeApi.DoQueryOrder(context.Background(), vcIds)
		if err != nil {
			fmt.Println("VC group query error:", err)
		}
		orderResults = append(orderResults, respVC...)
	}

	// 3) 回填本地数据库里的 orderId
	// 现在 orderResults 中包含 VV 和 VC 两组的查询结果
	for i := range orderResults {
		orderResult := &orderResults[i]
		order, err := h.svc.GetOrderByDownstreamOrderId(context.Background(), orderResult.DownstreamOrderId)
		if err != nil {
			// 没查到就跳过
			continue
		}
		orderResult.OrderId = order.GetOrderId()
	}

	// 4) 返回
	return SuccessJSON(c, orderResults)
}
