// internal/handler/gnc_handler.go
package handler

import (
	"strconv"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"
	"github.com/gofiber/fiber/v2"
)

type GncHandler struct {
	svc service.GncService
}

func NewGncHandler(svc service.GncService) *GncHandler {
	return &GncHandler{svc: svc}
}

// 注册路由
func (h *GncHandler) RegisterRoutes(r fiber.Router) {
	// /api/product/gift/gnc
	r.Post("/base", h.CreateGnc)
	r.Get("/base/:baseCode", h.GetGnc)
	r.Put("/base/:baseCode", h.UpdateGnc)
	r.Delete("/base/:baseCode", h.DeleteGnc)
	r.Post("/base/list", h.ListGnc) // 用 POST + body 或 GET + querystring 均可
	r.Post("/base/sync", h.SyncGncRemote)
}

func (h *GncHandler) CreateGnc(c *fiber.Ctx) error {
	var dto types.GncDTO
	if err := c.BodyParser(&dto); err != nil {
		return ErrorJSON(c, 400, err.Error())
	}
	created, err := h.svc.Create(&dto)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}
	return SuccessJSON(c, created)
}

func (h *GncHandler) GetGnc(c *fiber.Ctx) error {
	baseCode := c.Params("baseCode")
	data, err := h.svc.GetByBaseCode(baseCode)
	if err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, data)
}

func (h *GncHandler) UpdateGnc(c *fiber.Ctx) error {
	baseCode := c.Params("baseCode")
	var dto types.GncDTO
	if err := c.BodyParser(&dto); err != nil {
		return ErrorJSON(c, 400, err.Error())
	}
	updated, err := h.svc.UpdateByBaseCode(baseCode, &dto)
	if err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, updated)
}

func (h *GncHandler) DeleteGnc(c *fiber.Ctx) error {
	baseCode := c.Params("baseCode")
	if err := h.svc.DeleteByBaseCode(baseCode); err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, "Deleted")
}

func (h *GncHandler) ListGnc(c *fiber.Ctx) error {
	// 从 body 或 query 中获取 page/size
	var req ListRequest
	if err := c.BodyParser(&req); err != nil {
		// 若 body 没有，就从 query 里读
		pageQ := c.Query("page")
		sizeQ := c.Query("size")
		page, _ := strconv.ParseInt(pageQ, 10, 64)
		size, _ := strconv.ParseInt(sizeQ, 10, 64)
		req.Page = page
		req.Size = size
	}

	dataList, total, err := h.svc.List(req.Page, req.Size)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}

	// 统一返回 {code, message, data:{dataList, total}}
	respData := fiber.Map{
		"dataList": dataList,
		"total":    total,
	}
	return SuccessJSON(c, respData)
}

// SyncGncRemote 手动触发同步第三方数据
func (h *GncHandler) SyncGncRemote(c *fiber.Ctx) error {
	// 也可以让前端传 pageSize
	pageSize := 10
	remoteURL := "https://api0.10000hk.com/api/product/gift/public/list"
	tempToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzYxNDE3ODAsImlhdCI6MTczNTUzNjk4MCwiand0SGFzaCI6ImhLRExjd3NHLzFaN0JmSHgvQWlQNUE9PSIsInVzZXJTbiI6Inh6TUZXQ2dHMGdTbUNZRWVRU1dKT2pVdXJmM2JMdHpQa0YzTXpleEw3NjAifQ.bknLYTIr7O58-K21UwcpuDbvga8H0SaHNhrqtYscEJQ"

	// 调用 service
	if err := h.svc.SyncFromRemote(remoteURL, pageSize, tempToken); err != nil {
		return ErrorJSON(c, 500, err.Error())
	}
	return SuccessJSON(c, "Sync success")
}
