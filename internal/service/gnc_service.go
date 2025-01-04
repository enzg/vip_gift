// internal/service/gnc_service.go
package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/types"
)

type GncService interface {
	Create(dto *types.GncDTO) (*types.GncDTO, error)
	GetByBaseCode(baseCode string) (*types.GncDTO, error)
	UpdateByBaseCode(baseCode string, dto *types.GncDTO) (*types.GncDTO, error)
	DeleteByBaseCode(baseCode string) error

	List(page, size int64) ([]types.GncDTO, int64, error)
	SyncFromRemote(remoteURL string, pageSize int, bearerToken ...string) error
}

type gncServiceImpl struct {
	repo repository.GncRepo
}

func NewGncService(repo repository.GncRepo) GncService {
	return &gncServiceImpl{repo: repo}
}

// ---------------------
// 第三方接口返回结构示例
// ---------------------
type remoteGncResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		DataList []remoteGncProduct `json:"dataList"`
		Total    int                `json:"total"`
	} `json:"data"`
}
type remoteGncProduct struct {
	ProductId    string   `json:"publicCode"`
	ProductName  string   `json:"productName"`
	ProductType  int64    `json:"productType"`
	ParValue     float64  `json:"parValue"`
	SalePrice    float64  `json:"salePrice"`
	IsShelve     int64    `json:"status"`
	ProductDesc  string   `json:"desc"`
	ProductPics  []string `json:"pics"`
	ProductCover string   `json:"cover"`
}

func (s *gncServiceImpl) Create(dto *types.GncDTO) (*types.GncDTO, error) {
	if dto.BaseCode == "" {
		return nil, errors.New("baseCode is required")
	}
	ent, err := dto.ToEntity()
	if err != nil {
		return nil, err
	}
	// 默认上架
	if ent.IsShelve == 0 {
		ent.IsShelve = 1
	}
	if err := s.repo.CreateGnc(ent); err != nil {
		return nil, err
	}
	_ = dto.FromEntity(ent)
	return dto, nil
}

func (s *gncServiceImpl) GetByBaseCode(baseCode string) (*types.GncDTO, error) {
	ent, err := s.repo.GetGncByBaseCode(baseCode)
	if err != nil {
		return nil, err
	}
	var dto types.GncDTO
	_ = dto.FromEntity(ent)
	return &dto, nil
}

func (s *gncServiceImpl) UpdateByBaseCode(baseCode string, dto *types.GncDTO) (*types.GncDTO, error) {
	oldEnt, err := s.repo.GetGncByBaseCode(baseCode)
	if err != nil {
		return nil, err
	}
	// 这里简单处理：如果不为零就更新
	if dto.ProductName != "" {
		oldEnt.ProductName = dto.ProductName
	}
	if dto.ProductType != 0 {
		oldEnt.ProductType = dto.ProductType
	}
	if dto.SalePrice != 0 {
		oldEnt.SalePrice = dto.SalePrice
	}
	if dto.IsShelve != 0 {
		oldEnt.IsShelve = dto.IsShelve
	}
	if dto.ProductDesc != "" {
		oldEnt.ProductDesc = dto.ProductDesc
	}
	if dto.ProductCover != "" {
		oldEnt.ProductCover = dto.ProductCover
	}
	if dto.OriginData != "" {
		oldEnt.OriginData = dto.OriginData
	}

	if err := s.repo.UpdateGnc(oldEnt); err != nil {
		return nil, err
	}
	var updated types.GncDTO
	_ = updated.FromEntity(oldEnt)
	return &updated, nil
}

func (s *gncServiceImpl) DeleteByBaseCode(baseCode string) error {
	return s.repo.DeleteGncByBaseCode(baseCode)
}

func (s *gncServiceImpl) List(page, size int64) ([]types.GncDTO, int64, error) {
	ents, total, err := s.repo.ListGnc(page, size)
	if err != nil {
		return nil, 0, err
	}
	result := make([]types.GncDTO, len(ents))
	for i, e := range ents {
		_ = result[i].FromEntity(&e)
	}
	return result, total, nil
}

func (s *gncServiceImpl) SyncFromRemote(remoteURL string, pageSize int, bearerToken ...string) error {
	if pageSize <= 0 {
		pageSize = 5 // 默认每页抓取 5 条
	}
	page := 1

	client := &http.Client{
		Timeout: 10 * time.Second, // 超时时间可自行设定
	}

	// 若 Token 可选，通过 bearerToken 可变参数判断是否传入
	var token string
	if len(bearerToken) > 0 && bearerToken[0] != "" {
		token = bearerToken[0]
	}

	for {
		// 1) 构造请求体： { "page": X, "size": Y }
		reqBody := map[string]int{
			"page": page,
			"size": pageSize,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		// 2) 发送 POST 请求
		req, err := http.NewRequest("POST", remoteURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// 如果传入了 token，就加到 Header
		if token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to do request: %w", err)
		}
		// 使用完 resp 后记得关闭 body
		defer resp.Body.Close()
		// fmt.Printf("%s,%v", remoteURL, resp)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("remote returned non-200 status: %d", resp.StatusCode)
		}

		// 3) 解析返回 JSON
		var remoteResp remoteGncResponse
		if err := json.NewDecoder(resp.Body).Decode(&remoteResp); err != nil {
			return fmt.Errorf("failed to decode remote response: %w", err)
		}

		// 判断第三方响应 code
		if remoteResp.Code != 200 {
			return errors.New("remote response not success: " + remoteResp.Message)
		}

		// 4) 将 dataList 同步到本地 (Upsert)
		for _, p := range remoteResp.Data.DataList {
			err := s.upsertGncProduct(p)
			if err != nil {
				// 这里是你的业务逻辑决定: return error 或继续处理
				fmt.Printf("Warn: upsert %s failed: %v\n", p.ProductId, err)
			}
		}

		// 如果本页数据量小于 pageSize，说明已到最后一页，无需再翻
		gotCount := len(remoteResp.Data.DataList)
		if gotCount < pageSize {
			break
		}

		// 或者使用 total 判断:
		// if page*pageSize >= remoteResp.Data.Total { break }

		page++
	}

	return nil
}

// ---------------------
// upsertGncProduct
// ---------------------
// 根据 productId 去查本地数据库，如果已存在 => update，否则 => create
func (s *gncServiceImpl) upsertGncProduct(p remoteGncProduct) error {
	oldEnt, err := s.repo.GetGncByBaseCode(p.ProductId)
	if err != nil {
		// not found 或错误
		// 这里先简单判断是 "record not found" 还是别的错误
		// 如果是 record not found，说明需要 create
		// 否则返回 err
	}

	if oldEnt == nil || err != nil {
		// 需要 create
		newEnt := &types.GncEntity{
			BaseCode:     p.ProductId,
			ProductName:  p.ProductName,
			ProductType:  p.ProductType,
			ParValue:     p.ParValue,
			SalePrice:    p.SalePrice,
			IsShelve:     p.IsShelve,
			ProductDesc:  p.ProductDesc,
			ProductCover: p.ProductCover,
			// ProductPics 还未存储，需要你在 entity 里看如何持久化
			OriginData: "", // 如果想存下第三方原始字段，可序列化 p 到 JSON
		}
		return s.repo.CreateGnc(newEnt)
	} else {
		// update
		oldEnt.ProductName = p.ProductName
		oldEnt.ProductType = p.ProductType
		oldEnt.ParValue = p.ParValue
		oldEnt.SalePrice = p.SalePrice
		oldEnt.IsShelve = p.IsShelve
		oldEnt.ProductDesc = p.ProductDesc
		oldEnt.ProductCover = p.ProductCover
		// ...
		return s.repo.UpdateGnc(oldEnt)
	}
}
