package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"10000hk.com/vip_gift/internal/proxy"
	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/sink"
	"10000hk.com/vip_gift/internal/types"

	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
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

	r.Post("/orders/update_status", h.UpdateOrderStatus)
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
	case strings.Contains(req.DownstreamOrderId, "VF"):
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
		OrderId:    out.OrderId,
		Status:     int64(out.Status),
		StatusText: out.Status.String(),
		Message:    out.Status.Remark(),
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
	clientDto, _ := out.ToClientDTO()
	return SuccessJSON(c, clientDto)
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
	// to clientDto
	orderItems := lo.Map(items, func(item types.OrderDTO, idx int) *types.ClientOrderDTO {
		clientDto, _ := item.ToClientDTO()
		return clientDto
	})
	// 返回 { "total":..., "dataList": [...] }
	resp := fiber.Map{
		"total":    total,
		"dataList": orderItems,
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
	// 1) 按前缀 VV 和 VF 分组
	var vvIds []string
	var vcIds []string

	for _, oid := range req.OrderIds {
		switch {
		case strings.HasPrefix(oid, "VV"):
			vvIds = append(vvIds, oid)
		case strings.HasPrefix(oid, "VF"):
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
		if len(respVV) == 0 {
			for _, vvId := range vvIds {
				o, _ := h.svc.GetOrderByDownstreamOrderId(context.Background(), vvId)
				if o != nil {
					respVV = append(respVV, sink.OrderQueryResp{
						DownstreamOrderId: o.GetDownstreamOrderId(),
						OrderId:           o.GetOrderId(),
						Status:            int64(o.GetStatus()),
						StatusText:        o.GetStatus().String(),
						Remark:            o.GetStatus().Remark(),
					})
				}
			}
		}
		// 这里刷新一次本地数据库的订单状态
		for _, o := range respVV {
			// 从 respVV 中取出订单状态，更新到本地数据库
			order, err := h.svc.GetOrderByDownstreamOrderId(context.Background(), o.DownstreamOrderId)
			if err != nil {
				// 没查到就跳过
				continue
			}
			statusNew := types.OrderStatus(o.Status)
			dto := &types.OrderDTO{
				OrderId:           order.GetOrderId(),
				DownstreamOrderId: order.GetDownstreamOrderId(),
				Status:            statusNew,
				Remark:            statusNew.Remark(),
			}
			_ = h.svc.StoreToDB(context.Background(), dto)
		}
		orderResults = append(orderResults, respVV...)
	}

	// 2.2) 查询 VF 前缀订单
	if len(vcIds) > 0 {
		respVC, err := chargeApi.DoQueryOrder(context.Background(), vcIds)
		if err != nil {
			fmt.Println("VF group query error:", err)
		}
		if len(respVC) == 0 {
			for _, vcId := range vcIds {
				o, _ := h.svc.GetOrderByDownstreamOrderId(context.Background(), vcId)
				if o != nil {
					respVC = append(respVC, sink.OrderQueryResp{
						DownstreamOrderId: o.GetDownstreamOrderId(),
						OrderId:           o.GetOrderId(),
						Status:            int64(o.GetStatus()),
						StatusText:        o.GetStatus().String(),
						Remark:            o.GetStatus().Remark(),
					})
				}
			}
		}
		// 这里刷新一次本地数据库的订单状态
		for _, o := range respVC {
			// 从 respVV 中取出订单状态，更新到本地数据库
			order, err := h.svc.GetOrderByDownstreamOrderId(context.Background(), o.DownstreamOrderId)
			if err != nil {
				// 没查到就跳过
				continue
			}
			statusNew := types.OrderStatus(o.Status)
			dto := &types.OrderDTO{
				OrderId:           order.GetOrderId(),
				DownstreamOrderId: order.GetDownstreamOrderId(),
				Status:            statusNew,
				Remark:            statusNew.Remark(),
			}
			_ = h.svc.StoreToDB(context.Background(), dto)
		}
		orderResults = append(orderResults, respVC...)
	}

	// 3) 回填本地数据库里的 orderId
	// 现在 orderResults 中包含 VV 和 VF 两组的查询结果
	for i := range orderResults {
		orderResult := &orderResults[i]
		order, err := h.svc.GetOrderByDownstreamOrderId(context.Background(), orderResult.DownstreamOrderId)
		if err != nil {
			// 没查到就跳过
			continue
		}
		if orderResult.StatusText == "" {
			orderResult.StatusText = order.GetStatus().String()
		}
		if orderResult.Remark == "" {
			orderResult.Remark = order.GetStatus().Remark()
		}
		orderResult.OrderId = order.GetOrderId()
	}

	// 4) 返回
	return SuccessJSON(c, orderResults)
}

// UpdateOrderStatus 接口：发送 `order-update` 消息到 Kafka
func (h *OrderHandler) UpdateOrderStatus(c *fiber.Ctx) error {
	// 验证 userSn="CRM"
	userSn := c.Locals("userSn")
	if userSn != "CRM" {
		return ErrorJSON(c, http.StatusUnauthorized, "Unauthorized: userSn must be 'CRM'")
	}

	// 解析请求参数
	var req struct {
		OrderId           string `json:"orderId,omitempty"`
		DownstreamOrderId string `json:"downstreamOrderId,omitempty"`
		TradeStatus       string `json:"tradeStatus,omitempty"`
		RefundStatus      string `json:"refundStatus,omitempty"`
		DeliveryStatus    int64  `json:"deliveryStatus,omitempty"`
		SettlementStatus  int64  `json:"settlementStatus,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, http.StatusBadRequest, "Invalid request body")
	}

	// 如果没有 orderId，就尝试用 downstreamOrderId 查找
	if req.OrderId == "" && req.DownstreamOrderId != "" {
		order, err := h.svc.GetOrderByDownstreamOrderId(context.Background(), req.DownstreamOrderId)
		if err != nil {
			return ErrorJSON(c, http.StatusNotFound, fmt.Sprintf("Order not found for downstreamOrderId=%s", req.DownstreamOrderId))
		}
		req.OrderId = order.OrderId
	}

	if req.OrderId == "" {
		return ErrorJSON(c, http.StatusBadRequest, "Either orderId or downstreamOrderId is required")
	}

	// 发送 `order-update` 消息到 Kafka
	message, _ := json.Marshal(req)
	if err := h.svc.PublishOrderUpdate(context.Background(), req.DownstreamOrderId, message); err != nil {
		return ErrorJSON(c, http.StatusInternalServerError, "Failed to publish order update")
	}

	return SuccessJSON(c, req)
}
