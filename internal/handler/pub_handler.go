// internal/handler/pub_handler.go
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"
	"github.com/gofiber/fiber/v2"
)

type PubHandler struct {
	svc service.PubService
}

func NewPubHandler(svc service.PubService) *PubHandler {

	// 从 Service 获取全部分类
	cats, err := svc.GetAllCategories()
	if err != nil {
		log.Printf("[NewPubHandler] preload ephemeralMap from categories error: %v", err)
	} else {
		for _, cat := range cats {
			service.CateToSmallPositive(cat)
		}
		log.Printf("[NewPubHandler] ephemeralMap loaded with %d categories.\n", len(cats))
	}
	return &PubHandler{svc: svc}
}

func (h *PubHandler) RegisterRoutes(r fiber.Router) {
	// /api/product/gift/pub
	r.Get("/shop/one/:publicCode", h.GetPub)
	r.Post("/shop/search", h.SearchPub)
	r.Post("/charge/search", h.SearchChargePub)
	r.Post("/shop/categories", h.GetPubCategories)
	r.Post("/shop/list", h.ListPub)
	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		log.Fatal("jwtSecretKey environment variable is not set")
	}
	r.Use(JWTMiddleware(jwtSecretKey))
	// Existing endpoints
	r.Post("/public", h.CreatePub)
	r.Get("/public/one/:publicCode", h.GetPub)
	r.Put("/public/one/:publicCode", h.UpdatePub)
	r.Delete("/public/one/:publicCode", h.DeletePub)
	r.Post("/public/list", h.ListPub)

	// NEW endpoints for search
	r.Post("/public/search", h.SearchPub)
	r.Post("/public/categories", h.GetPubCategories)

	r.Post("/public/batch_category", h.BatchAddCategory)

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
func (h *PubHandler) SearchChargePub(c *fiber.Ctx) error {
	var req SearchRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, 400, "Invalid request body")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 10000
	}
	// 封装对 https://gift.10000hk.com/api/charge/product/list 的请求
	// 将请求数据转换为 JSON 格式
	payload, err := json.Marshal(req)
	if err != nil {
		return ErrorJSON(c, 500, "Failed to marshal request")
	}

	// 构造 HTTP POST 请求
	apiURL := "https://gift.10000hk.com/api/charge/product/list"
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
	if err != nil {
		return ErrorJSON(c, 500, "Failed to create HTTP request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 设置超时时间
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return ErrorJSON(c, 500, "Failed to perform HTTP request")
	}
	defer resp.Body.Close()

	// 读取响应数据
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ErrorJSON(c, 500, "Failed to read response body")
	}

	// 如果 HTTP 状态码不是 200，直接返回错误
	if resp.StatusCode != http.StatusOK {
		return ErrorJSON(c, resp.StatusCode, "Request to external API failed")
	}

	// 定义 API 返回数据的结构体
	type DataItem struct {
		Platform  string `json:"platform"`
		Product   string `json:"product"`
		Range     string `json:"range"`
		SalePrice string `json:"salePrice"`
		ProductId string `json:"productId"`
	}

	type APIResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			DataList []DataItem `json:"dataList"`
			Total    int        `json:"total"`
		} `json:"data"`
	}

	// 解析 API 返回的 JSON 数据
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return ErrorJSON(c, 500, "Failed to parse API response")
	}

	// 如果外部 API 返回的 code 不是 200，则返回错误信息
	if apiResp.Code != 200 {
		return ErrorJSON(c, apiResp.Code, apiResp.Message)
	}

	// 如果请求中传入了 productIds，则过滤返回的结果，只保留符合条件的数据
	if len(req.ProductIds) > 0 {
		// 建立一个 map 便于查找
		productMap := make(map[string]bool)
		for _, pid := range req.ProductIds {
			productMap[pid] = true
		}
		filteredList := make([]DataItem, 0)
		for _, item := range apiResp.Data.DataList {
			if productMap[item.ProductId] {
				filteredList = append(filteredList, item)
			}
		}
		apiResp.Data.DataList = filteredList
		apiResp.Data.Total = len(filteredList)
	}

	// 成功则将 data 部分直接返回给前端
	return SuccessJSON(c, apiResp.Data)

}
func (h *PubHandler) SearchPub(c *fiber.Ctx) error {
	var req SearchRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, 400, "Invalid request body")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 10000
	}
	var cateMap = service.DumpCateReverse()
	keyword := cateMap[req.Cate]
	results, total, err := h.svc.SearchByKeyword(keyword, req.Page, req.Size)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}

	return SuccessJSON(c, fiber.Map{
		"total": total,
		"dataList": fiber.Map{
			"title": keyword,
			"items": results,
		},
	})
}

// POST /public/categories
// Body: {} (若不需要参数)
func (h *PubHandler) GetPubCategories(c *fiber.Ctx) error {
	// 1) 调用 Service 获取 categories 字符串切片
	cats, err := h.svc.GetAllCategories()
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}

	// 2) 将 cats 转换成 [{ "cate": string, "id": int64 }, ...]
	dataList := make([]map[string]interface{}, 0, len(cats))
	for _, cat := range cats {
		id, _ := service.CateToSmallPositive(cat)
		dataList = append(dataList, map[string]interface{}{
			"cate": cat,
			"id":   id, // 这里随意给个不重复的ID，如自增
		})
	}

	// 3) 返回给前端
	return SuccessJSON(c, fiber.Map{
		"dataList": dataList,
		"total":    len(dataList),
	})
}

func (h *PubHandler) BatchAddCategory(c *fiber.Ctx) error {
	var req BatchCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorJSON(c, 400, "invalid request body")
	}
	if req.Prefix == "" || req.Category == "" || req.Tag == "" {
		return ErrorJSON(c, 400, "prefix & category & tag are required")
	}

	// 调用 Service
	err := h.svc.BatchAddCategoryForPrefix(req.Prefix, req.Category, req.Tag)
	if err != nil {
		return ErrorJSON(c, 500, err.Error())
	}

	return SuccessJSON(c, fmt.Sprintf("已批量为 prefix=%q 的产品添加分类=%q", req.Prefix, req.Category))
}
