// internal/handler/pub_handler.go
package handler

import (
	"strconv"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"
	"github.com/gofiber/fiber/v2"
)

type PubHandler struct {
	svc service.PubService
}

func NewPubHandler(svc service.PubService) *PubHandler {
	return &PubHandler{svc: svc}
}

func (h *PubHandler) RegisterRoutes(r fiber.Router) {
	// /api/product/gift/pub
	r.Post("/public", h.CreatePub)
	r.Get("/public/:publicCode", h.GetPub)
	r.Put("/public/:publicCode", h.UpdatePub)
	r.Delete("/public/:publicCode", h.DeletePub)
	r.Post("/public/list", h.ListPub)
}

func (h *PubHandler) CreatePub(c *fiber.Ctx) error {
	var dto types.PubDTO
	if err := c.BodyParser(&dto); err != nil {
		return ErrorJSON(c, 400, err.Error())
	}
	created, err := h.svc.Create(&dto)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}
	return SuccessJSON(c, created)
}

func (h *PubHandler) GetPub(c *fiber.Ctx) error {
	publicCode := c.Params("publicCode")
	got, err := h.svc.GetByPublicCode(publicCode)
	if err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, got)
}

func (h *PubHandler) UpdatePub(c *fiber.Ctx) error {
	publicCode := c.Params("publicCode")
	var dto types.PubDTO
	if err := c.BodyParser(&dto); err != nil {
		return ErrorJSON(c, 400, err.Error())
	}
	updated, err := h.svc.UpdateByPublicCode(publicCode, &dto)
	if err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, updated)
}

func (h *PubHandler) DeletePub(c *fiber.Ctx) error {
	publicCode := c.Params("publicCode")
	if err := h.svc.DeleteByPublicCode(publicCode); err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, "Deleted")
}

func (h *PubHandler) ListPub(c *fiber.Ctx) error {
	var req ListRequest
	if err := c.BodyParser(&req); err != nil {
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
	respData := fiber.Map{
		"dataList": dataList,
		"total":    total,
	}
	return SuccessJSON(c, respData)
}
