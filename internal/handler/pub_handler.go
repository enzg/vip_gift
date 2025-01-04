// internal/handler/pub_handler.go
package handler

import (
	"fmt"
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

func (h *PubHandler) RegisterRoutes(r fiber.Router, noAuth fiber.Router) {
	// /api/product/gift/pub

	// Existing endpoints
	r.Post("/public", h.CreatePub)
	r.Get("/public/one/:publicCode", h.GetPub)
	r.Put("/public/one/:publicCode", h.UpdatePub)
	r.Delete("/public/one/:publicCode", h.DeletePub)
	r.Post("/public/list", h.ListPub)

	// NEW endpoints for search
	r.Get("/public/search", h.SearchPub)
	r.Get("/public/categories", h.GetPubCategories)

	r.Post("/public/batch_category", h.BatchAddCategory)
	
	noAuth.Get("/public/one/:publicCode", h.GetPub)
	noAuth.Get("/public/search", h.SearchPub)
	noAuth.Get("/public/categories", h.GetPubCategories)
}

// -------------------------------------------------------------------
// Create
// -------------------------------------------------------------------
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

// -------------------------------------------------------------------
// Get
// -------------------------------------------------------------------
func (h *PubHandler) GetPub(c *fiber.Ctx) error {
	publicCode := c.Params("publicCode")
	got, err := h.svc.GetByPublicCode(publicCode)
	if err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, got)
}

// -------------------------------------------------------------------
// Update
// -------------------------------------------------------------------
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

// -------------------------------------------------------------------
// Delete
// -------------------------------------------------------------------
func (h *PubHandler) DeletePub(c *fiber.Ctx) error {
	publicCode := c.Params("publicCode")
	if err := h.svc.DeleteByPublicCode(publicCode); err != nil {
		return ErrorJSON(c, 404, err.Error())
	}
	return SuccessJSON(c, "Deleted")
}

// -------------------------------------------------------------------
// List
// -------------------------------------------------------------------
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

// -------------------------------------------------------------------
// NEW: Search by keyword
// GET /public/search?keyword=xxx&page=1&size=10
// -------------------------------------------------------------------
func (h *PubHandler) SearchPub(c *fiber.Ctx) error {
	keyword := c.Query("keyword", "")
	pageQ := c.Query("page", "1")
	sizeQ := c.Query("size", "10")

	page, _ := strconv.ParseInt(pageQ, 10, 64)
	size, _ := strconv.ParseInt(sizeQ, 10, 64)

	results,total, err := h.svc.SearchByKeyword(keyword, page, size)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}
	return SuccessJSON(c, fiber.Map{
		"keyword": keyword,
		"total":   total,
		"dataList":    results,
	})
}

// -------------------------------------------------------------------
// NEW: Get categories list from ES
// GET /public/categories
// -------------------------------------------------------------------
func (h *PubHandler) GetPubCategories(c *fiber.Ctx) error {
	cats, err := h.svc.GetAllCategories()
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}
	return SuccessJSON(c, fiber.Map{
		"dataList": cats,
		"total":      len(cats),
	})
}

func (h *PubHandler) BatchAddCategory(c *fiber.Ctx) error {
	var req BatchCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, 400, "invalid request body")
	}
	if req.Prefix == "" || req.Category == "" {
		return ErrorJSON(c, 400, "prefix & category are required")
	}

	// 调用 Service
	err := h.svc.BatchAddCategoryForPrefix(req.Prefix, req.Category)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}

	return SuccessJSON(c, fmt.Sprintf("已批量为 prefix=%q 的产品添加分类=%q", req.Prefix, req.Category))
}
