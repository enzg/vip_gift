// internal/service/pub_service.go

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/types"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// PubService 定义对外的接口
type PubService interface {
	// ----- Existing Methods -----
	Create(dto *types.PubDTO) (*types.PubDTO, error)
	GetByPublicCode(publicCode string) (*types.PubDTO, error)
	UpdateByPublicCode(publicCode string, dto *types.PubDTO) (*types.PubDTO, error)
	DeleteByPublicCode(publicCode string) error
	List(page, size int64) ([]types.PubDTO, int64, error)

	// ----- Newly Added Methods -----
	SearchByKeyword(keyword string, page, size int64) ([]types.PubDTO, error)
	GetAllCategories() ([]string, error)
	BatchAddCategoryForPrefix(string, string) error
}

type pubServiceImpl struct {
	repo repository.PubRepo
	es   *elasticsearch.Client
}

// NewPubService 返回默认的 pubServiceImpl 实例
func NewPubService(repo repository.PubRepo, es *elasticsearch.Client) PubService {
	return &pubServiceImpl{repo: repo, es: es}
}

// -------------------------------------------------------------------
// 1) Create
// -------------------------------------------------------------------
func (s *pubServiceImpl) Create(dto *types.PubDTO) (*types.PubDTO, error) {
	if dto.PublicCode == "" {
		return nil, errors.New("publicCode is required")
	}
	ent, err := dto.ToEntity()
	if err != nil {
		return nil, err
	}
	// 默认上架
	if ent.Status == 0 {
		ent.Status = 1
	}

	// 1) 写数据库
	if err := s.repo.CreatePub(ent); err != nil {
		return nil, err
	}

	// 2) 同步到 ES
	if err := s.indexToES(ent); err != nil {
		// 这里视需求决定：要不要回滚 DB？还是仅警告
		return nil, fmt.Errorf("failed to index to ES: %w", err)
	}

	_ = dto.FromEntity(ent)
	return dto, nil
}

// -------------------------------------------------------------------
// 2) Get
// -------------------------------------------------------------------
func (s *pubServiceImpl) GetByPublicCode(publicCode string) (*types.PubDTO, error) {
	ent, err := s.repo.GetPubByPublicCode(publicCode)
	if err != nil {
		return nil, err
	}
	var dto types.PubDTO
	_ = dto.FromEntity(ent)
	return &dto, nil
}

// -------------------------------------------------------------------
// 3) Update
// -------------------------------------------------------------------
func (s *pubServiceImpl) UpdateByPublicCode(publicCode string, dto *types.PubDTO) (*types.PubDTO, error) {
	// 1) 先查旧记录
	oldEnt, err := s.repo.GetPubByPublicCode(publicCode)
	if err != nil {
		return nil, err
	}

	// 2) 用 dto 覆盖旧记录字段
	if dto.SalePrice != 0 {
		oldEnt.SalePrice = dto.SalePrice
	}
	if dto.ParValue != 0 {
		oldEnt.ParValue = dto.ParValue
	}
	if dto.Cover != "" {
		oldEnt.Cover = dto.Cover
	}
	if dto.Desc != "" {
		oldEnt.Desc = dto.Desc
	}
	if dto.OriginData != "" {
		oldEnt.OriginData = dto.OriginData
	}
	if dto.Status != 0 {
		oldEnt.Status = dto.Status
	}

	// 更新组合(Compositions)
	if len(dto.Compositions) > 0 {
		newComps := make([]types.PubComposeEntity, len(dto.Compositions))
		for i, cDto := range dto.Compositions {
			newComps[i].BaseCode = cDto.BaseCode
			newComps[i].Strategy = cDto.Strategy
			newComps[i].Snapshot = cDto.Snapshot
		}
		oldEnt.Compositions = newComps
	} else {
		// 这里视业务：若前端传空数组, 可能表示清空
		oldEnt.Compositions = nil
	}

	// 3) 更新数据库
	if err := s.repo.UpdatePub(oldEnt); err != nil {
		return nil, err
	}

	// 4) 同步到 ES
	if err := s.indexToES(oldEnt); err != nil {
		return nil, fmt.Errorf("ES index error: %w", err)
	}

	// 5) 返回更新后的 dto
	var updated types.PubDTO
	_ = updated.FromEntity(oldEnt)
	return &updated, nil
}

// -------------------------------------------------------------------
// 4) Delete
// -------------------------------------------------------------------
func (s *pubServiceImpl) DeleteByPublicCode(publicCode string) error {
	// 1) 先查实体
	ent, err := s.repo.GetPubByPublicCode(publicCode)
	if err != nil {
		return err
	}

	// 2) 删 DB
	if err := s.repo.DeletePubByPublicCode(publicCode); err != nil {
		return err
	}

	// 3) 删 ES
	if err := s.deleteFromES(ent); err != nil {
		// 看你要不要回滚 DB
		return fmt.Errorf("failed to delete from ES: %w", err)
	}
	return nil
}

// -------------------------------------------------------------------
// 5) List (reads from DB, not from ES)
// -------------------------------------------------------------------
func (s *pubServiceImpl) List(page, size int64) ([]types.PubDTO, int64, error) {
	ents, total, err := s.repo.ListPub(page, size)
	if err != nil {
		return nil, 0, err
	}
	result := make([]types.PubDTO, len(ents))
	for i, e := range ents {
		_ = result[i].FromEntity(&e)
	}
	return result, total, nil
}

// -------------------------------------------------------------------
// 6) Search in ES by keyword (NEW)
// -------------------------------------------------------------------
func (s *pubServiceImpl) SearchByKeyword(keyword string, page, size int64) ([]types.PubDTO, error) {
	// Build a match query on "name" field
	from := (page - 1) * size
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"name": keyword,
			},
		},
		"from": from,
		"size": size,
		// You can also add "sort" here, e.g. [{"_score":{"order":"desc"}}]
	}

	bodyBytes, _ := json.Marshal(query)
	reqES := esapi.SearchRequest{
		Index: []string{"products_index"}, // your index
		Body:  bytes.NewReader(bodyBytes),
	}
	resp, err := reqES.Do(context.Background(), s.es.Transport)
	if err != nil {
		return nil, fmt.Errorf("ES search error: %v", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("ES search status: %s", resp.Status())
	}

	// Parse response
	var sr map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	hits := sr["hits"].(map[string]interface{})["hits"].([]interface{})
	var results []types.PubDTO
	for _, h := range hits {
		doc := h.(map[string]interface{})
		source := doc["_source"].(map[string]interface{})

		// Convert source to PubDTO
		// We'll just do it manually here, or you can unmarshal JSON
		dto := types.PubDTO{
			PublicCode:  stringValue(source["id"]),
			ProductName: stringValue(source["name"]),
		}

		// If you have categories in your PubDTO as well
		if cArr, ok := source["categories"].([]interface{}); ok {
			cats := make([]string, 0, len(cArr))
			for _, cVal := range cArr {
				if sVal, ok := cVal.(string); ok {
					cats = append(cats, sVal)
				}
			}
			dto.Categories = cats
		}

		results = append(results, dto)
	}

	return results, nil
}

// -------------------------------------------------------------------
// 7) Get distinct categories via ES Aggregation (NEW)
// -------------------------------------------------------------------
func (s *pubServiceImpl) GetAllCategories() ([]string, error) {
	// Build a terms aggregation query
	query := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			"catAgg": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "categories",
					"size":  10000, // adjust as needed
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(query)
	reqES := esapi.SearchRequest{
		Index: []string{"products_index"},
		Body:  bytes.NewReader(bodyBytes),
	}
	resp, err := reqES.Do(context.Background(), s.es.Transport)
	if err != nil {
		return nil, fmt.Errorf("ES agg error: %v", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("ES status: %s", resp.Status())
	}

	var sr map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	aggs := sr["aggregations"].(map[string]interface{})
	catAgg := aggs["catAgg"].(map[string]interface{})
	buckets := catAgg["buckets"].([]interface{})

	var categories []string
	for _, b := range buckets {
		bucket := b.(map[string]interface{})
		key := bucket["key"].(string)
		categories = append(categories, key)
	}

	return categories, nil
}

// ========================= Elasticsearch Helper Methods =========================

// indexToES 把 pubEntity 同步到 ES
func (s *pubServiceImpl) indexToES(ent *types.PubEntity) error {
	doc := map[string]interface{}{
		"id":         ent.PublicCode, // _id
		"name":       ent.ProductName,
		"categories": ent.Categories, // 需要在 PubEntity 中有
		"created_at": time.Now().Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	bodyBytes, _ := json.Marshal(doc)

	reqES := esapi.IndexRequest{
		Index:      "products_index", // 你的索引名
		DocumentID: ent.PublicCode,
		Body:       bytes.NewReader(bodyBytes),
		Refresh:    "true", // dev环境可用, 生产可去掉
	}
	resp, err := reqES.Do(context.Background(), s.es.Transport)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("ES index error: %s", resp.Status())
	}
	return nil
}

func (s *pubServiceImpl) deleteFromES(ent *types.PubEntity) error {
	reqES := esapi.DeleteRequest{
		Index:      "products_index",
		DocumentID: ent.PublicCode,
		Refresh:    "true",
	}
	resp, err := reqES.Do(context.Background(), s.es.Transport)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		// 如果是 404, 可算正常
		if resp.StatusCode != 404 {
			return fmt.Errorf("ES delete error: %s", resp.Status())
		}
	}
	return nil
}

// stringValue is a helper to safely convert an interface{} to a string
func stringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
func (s *pubServiceImpl) BatchAddCategoryForPrefix(prefix, category string) error {
	// 1) 从 DB 中查出所有名称以 prefix 开头的 Pub
	var pubs []types.PubEntity
	if err := s.repo.FindPubByNamePrefix(prefix, &pubs); err != nil {
		return err
	}
	if len(pubs) == 0 {
		log.Printf("没有找到任何名称以 %q 开头的产品", prefix)
		return nil
	}

	// 2) 给每个 pubEntity 的 Categories 追加 category, 并同步到 ES
	for i := range pubs {
		pub := &pubs[i]
		// 如果已经包含就不重复添加
		if !containsString(pub.Categories, category) {
			pub.Categories = append(pub.Categories, category)
		}
		// 方式1: 仅写ES(如果 Categories 不存MySQL):
		if err := s.indexToES(pub); err != nil {
			log.Printf("[WARN] indexToES failed for %s: %v\n", pub.PublicCode, err)
			// 也可选择return err看你业务需求
		}

		// 方式2(可选): 如果你希望 DB 也存 categories_json,
		//   则可以通过 UpdatePub(...) 触发 DB+ES 同步:
		// var dto types.PubDTO
		// _ = dto.FromEntity(pub)
		// if _, err := s.UpdateByPublicCode(pub.PublicCode, &dto); err != nil {
		//     log.Printf("[WARN] UpdateByPublicCode failed for %s: %v\n", pub.PublicCode, err)
		// }
	}

	log.Printf("已为 %d 个产品追加分类 %q 并同步ES", len(pubs), category)
	return nil
}

func containsString(arr []string, val string) bool {
	for _, a := range arr {
		if a == val {
			return true
		}
	}
	return false
}