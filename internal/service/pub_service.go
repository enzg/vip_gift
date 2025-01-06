// internal/service/pub_service.go

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
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
	SearchByKeyword(keyword string, page, size int64) ([]GroupedItem, int64, error)
	GetAllCategories() ([]string, error)
	BatchAddCategoryForPrefix(string, string, string) error
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
	if len(dto.Categories) > 0 {
		oldEnt.Categories = dto.Categories
	}
	if len(dto.Pics) > 0 {
		oldEnt.Pics = dto.Pics
	}
	if dto.Tag != "" {
		oldEnt.Tag = dto.Tag
	}
	if dto.ProductName != "" {
		oldEnt.ProductName = dto.ProductName
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

// GroupedItem 用于返回给调用者
type GroupedItem struct {
	Name string         `json:"name"`
	Card []types.PubDTO `json:"card"`
}

// SearchByKeyword: 从 ES 中按分类(`keyword`)搜索,
// 如果某条数据发现字段缺失, 则回查DB并更新ES.

func (s *pubServiceImpl) SearchByKeyword(keyword string, page, size int64) ([]GroupedItem, int64, error) {
	from := (page - 1) * size

	// 1) 构建查询
	// 在 “term” 查询基础上，新增 "sort": [{"parValue": {"order": "asc"}}]
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"categories": keyword,
			},
		},
		"from": from,
		"size": size,
		"sort": []interface{}{
			map[string]interface{}{
				"parValue": map[string]interface{}{
					"order": "asc",
				},
			},
		},
	}

	// 2) 发送 SearchRequest
	bodyBytes, _ := json.Marshal(query)
	reqES := esapi.SearchRequest{
		Index: []string{"vip_pub"}, // 你的 ES 索引名
		Body:  bytes.NewReader(bodyBytes),
	}
	resp, err := reqES.Do(context.Background(), s.es.Transport)
	if err != nil {
		return nil, 0, fmt.Errorf("ES search error: %v", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, 0, fmt.Errorf("ES search status: %s", resp.Status())
	}

	// 3) 解析返回
	var sr map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, 0, err
	}

	// 4) 提取 totalHits
	var totalHits int64
	if hitsVal, ok := sr["hits"].(map[string]interface{}); ok {
		if totalObj, ok2 := hitsVal["total"].(map[string]interface{}); ok2 {
			if val, ok3 := totalObj["value"].(float64); ok3 {
				totalHits = int64(val)
			}
		}
	}

	// 5) 提取文档 hits
	hitsArr, _ := sr["hits"].(map[string]interface{})["hits"].([]interface{})
	groupMap := make(map[string][]types.PubDTO)

	for _, h := range hitsArr {
		doc := h.(map[string]interface{})
		src := doc["_source"].(map[string]interface{})

		dto := types.PubDTO{
			PublicCode:  stringValue(src["id"]),
			ProductName: stringValue(src["name"]),
			SalePrice:   floatValue(src["salePrice"]),
			ParValue:    floatValue(src["parValue"]),
			Cover:       stringValue(src["cover"]),
			Categories:  stringSliceValue(src["categories"]),
			Pics:        stringSliceValue(src["pics"]),
			Desc:        stringValue(src["desc"]),
			Tag:         stringValue(src["tag"]),
			Fetched:     boolValue(src["fetched"]),
		}

		// 如果要做 isIncomplete / fillFromDBAndUpdateES，就保留逻辑
		if isIncomplete(dto) {
			if err := s.fillFromDBAndUpdateES(&dto); err != nil {
				log.Printf("[WARN] fillFromDBAndUpdateES fail for %s: %v\n", dto.PublicCode, err)
			}
		}
		groupMap[dto.Tag] = append(groupMap[dto.Tag], dto)
	}

	// 6) 分组输出
	var final []GroupedItem
	for tag, items := range groupMap {
		final = append(final, GroupedItem{
			Name: tag,
			Card: items,
		})
	}
	return final, totalHits, nil
}

// -------------------- 辅助函数: 检查字段是否不完整 --------------------
func isIncomplete(dto types.PubDTO) bool {
	if dto.Fetched {
		return false
	}
	// 示例: 如果 cover/pics为空 或 salePrice=0 认为不完整
	if dto.Cover == "" || len(dto.Pics) == 0 || dto.SalePrice == 0 || dto.Desc == "" {
		return true
	}
	// 你也可加更多逻辑: name==""、categories 为空等
	return false
}

// -------------------- 回查DB并更新ES --------------------
func (s *pubServiceImpl) fillFromDBAndUpdateES(dto *types.PubDTO) error {
	ent, err := s.repo.GetPubByPublicCode(dto.PublicCode)
	if err != nil {
		return fmt.Errorf("GetPubByPublicCode fail: %w", err)
	}
	var dbDTO types.PubDTO
	_ = dbDTO.FromEntity(ent)

	// 如果 dbDTO 也是空, 说明确实无数据
	// => 标识这个文档已检查过(空)
	// => 下次就不再重复
	allEmpty := dbDTO.Cover == "" && len(dbDTO.Pics) == 0 && dbDTO.SalePrice == 0
	if allEmpty {
		// 你可以用 "fetched" 或 "final" 字段表示
		dto.Fetched = true
	} else {
		// 	// 仅当 ES 里的字段为空时, 才用DB覆盖
		if dto.Cover == "" {
			dto.Cover = dbDTO.Cover
		}
		if len(dto.Pics) == 0 {
			dto.Pics = dbDTO.Pics
		}
		if dto.SalePrice == 0 {
			dto.SalePrice = dbDTO.SalePrice
		}
		if dto.ParValue == 0 {
			dto.ParValue = dbDTO.ParValue
		}
		if dto.ProductName == "" {
			dto.ProductName = dbDTO.ProductName
		}
		if len(dto.Categories) == 0 {
			dto.Categories = dbDTO.Categories
		}
		if dto.Tag == "" {
			dto.Tag = dbDTO.Tag
		}
		if dto.Desc == "" {
			dto.Desc = dbDTO.Desc
		}

		dto.Fetched = dto.Fetched || allEmpty
	}

	// 重新写回 ES
	newEnt, _ := dto.ToEntity()
	return s.indexToES(newEnt)
}

// -------------------------------------------------------------------
// 7) Get distinct categories via ES Aggregation (NEW)
// -------------------------------------------------------------------
// func (s *pubServiceImpl) GetAllCategories() ([]string, error) {
// 	// Build a terms aggregation query
// 	query := map[string]interface{}{
// 		"size": 0,
// 		"aggs": map[string]interface{}{
// 			"catAgg": map[string]interface{}{
// 				"terms": map[string]interface{}{
// 					"field": "categories",
// 					"size":  10000, // adjust as needed
// 				},
// 			},
// 		},
// 	}

// 	bodyBytes, _ := json.Marshal(query)
// 	reqES := esapi.SearchRequest{
// 		Index: []string{"vip_pub"},
// 		Body:  bytes.NewReader(bodyBytes),
// 	}
// 	resp, err := reqES.Do(context.Background(), s.es.Transport)
// 	if err != nil {
// 		return nil, fmt.Errorf("ES agg error: %v", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.IsError() {
// 		return nil, fmt.Errorf("ES status: %s", resp.Status())
// 	}

// 	var sr map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
// 		return nil, err
// 	}

// 	aggs := sr["aggregations"].(map[string]interface{})
// 	catAgg := aggs["catAgg"].(map[string]interface{})
// 	buckets := catAgg["buckets"].([]interface{})

// 	var categories []string
// 	for _, b := range buckets {
// 		bucket := b.(map[string]interface{})
// 		key := bucket["key"].(string)
// 		categories = append(categories, key)
// 	}

//		return categories, nil
//	}
var defaultCates = []string{
	"视频会员",
	"音乐会员",
	"阅读听书",
	"网络工具",
	"休闲生活",
	"外卖商超",
	"美食饮品",
	"交通出行",
	"腾讯QQ",
}

func (s *pubServiceImpl) GetAllCategories() ([]string, error) {
	// 1) 拉取 ES 中分类
	esCats, err := s.fetchEsCategories()
	if err != nil {
		return nil, err
	}

	// 2) 把 9 个默认分类放进 final, 顺序不变
	final := make([]string, len(defaultCates))
	copy(final, defaultCates)

	// 用一个 set 来记录这 9 个，避免跟 ES 重复
	used := make(map[string]bool, len(defaultCates))
	for _, c := range defaultCates {
		used[c] = true
	}

	// 3) 遍历 esCats, 若不在 used 中, 追加到 final
	for _, cat := range esCats {
		if !used[cat] {
			final = append(final, cat)
			used[cat] = true
		}
	}

	return final, nil
}

// ========================= Elasticsearch Helper Methods =========================
func (s *pubServiceImpl) fetchEsCategories() ([]string, error) {
	query := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			"catAgg": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "categories",
					"size":  10000,
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(query)
	reqES := esapi.SearchRequest{
		Index: []string{"vip_pub"},
		Body:  bytes.NewReader(bodyBytes),
	}
	resp, err := reqES.Do(context.Background(), s.es.Transport)
	if err != nil {
		return nil, fmt.Errorf("ES agg error: %w", err)
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

	var cats []string
	for _, b := range buckets {
		bucket := b.(map[string]interface{})
		key := bucket["key"].(string)
		cats = append(cats, key)
	}
	return cats, nil
}

// indexToES 把 pubEntity 同步到 ES
func (s *pubServiceImpl) indexToES(ent *types.PubEntity) error {
	doc := map[string]interface{}{
		"id":         ent.PublicCode, // _id
		"name":       ent.ProductName,
		"tag":        ent.Tag,
		"categories": ent.Categories, // 需要在 PubEntity 中有
		"salePrice":  ent.SalePrice,
		"parValue":   ent.ParValue,
		"cover":      ent.Cover,
		"pics":       ent.Pics,
		"fetched":    ent.Fetched,
		"created_at": time.Now().Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	bodyBytes, _ := json.Marshal(doc)

	reqES := esapi.IndexRequest{
		Index:      "vip_pub", // 你的索引名
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
		Index:      "vip_pub",
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
func boolValue(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
func (s *pubServiceImpl) BatchAddCategoryForPrefix(prefix, category string, tag string) error {
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
		pub.Tag = tag
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
func floatValue(v interface{}) float64 {
	if v == nil {
		return 0.0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	// 如果还想兼容字符串->数字，可以做Parse
	// log warn or silently ignore
	return 0.0
}

// stringSliceValue 尝试将 v 转换为 []string，如果出错或不是字符串数组，就返回空切片
func stringSliceValue(v interface{}) []string {
	// 先断言成 []interface{}
	arr, ok := v.([]interface{})
	if !ok {
		// 若不是 []interface{}，说明类型不符合
		return nil
	}

	// 遍历 arr，把每个项若是 string 则放到 results
	results := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok2 := item.(string); ok2 {
			results = append(results, s)
		}
	}
	return results
}
func containsString(arr []string, val string) bool {
	for _, a := range arr {
		if a == val {
			return true
		}
	}
	return false
}

// 内存中的cate字典：cate → id
var ephemeralMap = make(map[string]int64)
var nextID int64 = 1

const maxCateCount = 1000 // 设定分类上限

// 为了线程安全，使用一个互斥锁
var cateLock sync.Mutex

// CateToSmallPositive 不可逆，只分配正整数(1,2,3...)
// 当新cate出现时，如果超过 maxCateCount 则返回错误
func CateToSmallPositive(cate string) (int64, error) {
	cateLock.Lock()
	defer cateLock.Unlock()

	// 如果已经分配过，直接返回
	if id, ok := ephemeralMap[cate]; ok {
		return id, nil
	}

	// 若还没出现过，但已到达上限
	if nextID > maxCateCount {
		return 0, fmt.Errorf("分类已达上限(%d)，无法为 %q 分配新的ID", maxCateCount, cate)
	}

	ephemeralMap[cate] = nextID
	nextID++
	return ephemeralMap[cate], nil
}

// DumpCateReverse 输出一个 map[int64]string，表示已分配的 (id -> cate)
func DumpCateReverse() map[int64]string {
	cateLock.Lock()
	defer cateLock.Unlock()

	reverseMap := make(map[int64]string, len(ephemeralMap))
	for c, id := range ephemeralMap {
		reverseMap[id] = c
	}
	return reverseMap
}
