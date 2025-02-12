package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/sink"
	"10000hk.com/vip_gift/internal/types"
)

type giftApiImpl struct {
	upstreamURL map[string]string
	pub         service.PubService
	httpClient  *http.Client
}

func NewGiftApi(upstreamURL map[string]string, pubSvc service.PubService) types.OrderApi {
	return &giftApiImpl{
		upstreamURL: upstreamURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		pub: pubSvc,
	}
}

func (api *giftApiImpl) DoSendSms(ctx context.Context, req sink.SmsReq) (*sink.OrderCreateResp, error) {
	// TODO: implement if needed
	return &sink.OrderCreateResp{}, nil
}
func (api *giftApiImpl) ToOrderDto(ctx context.Context, ent sink.OrderCreateReq) (types.OrderDTO, error) {
	// 保证 downstreamOrderId 不为空 这个字段是在不同节点查询订单用的
	// 正式版本应该是转变成公钥加密的数据
	var downstreamOrderId string = ent.DownstreamOrderId
	if downstreamOrderId == "" {
		return types.OrderDTO{}, fmt.Errorf("ToOrderDto: downstreamOrderId is required")
	}
	// 检查publicCode 对应的产品有没有
	pubCode := ent.PublicCode
	pub, err := api.pub.GetByPublicCode(pubCode)
	if err != nil {
		log.Printf("[ToOrderDto] GetByPublicCode error: %v\n", err)
		return types.OrderDTO{}, err
	}

	packReq := sink.BizDataJSON[sink.OrderCreateReq]{
		Body:  ent,
		Extra: ent.DataJSON,
	}
	bizReqJSON, _ := json.Marshal(packReq)
	dto := types.OrderDTO{
		DownstreamOrderId: downstreamOrderId,
		DataJSON:          string(bizReqJSON),
		Status:            0,
		Remark:            "",
		CommissionRule:    "MF", // 权益业务通通默认秒返
		UserSn:            ent.PartnerId,
		ParentSn:          ent.ParentSn,
		PublicCode:        pubCode,
		CommissionSelf:    pub.CommissionMF * 0.85,
		CommissionParent:  pub.CommissionMF * 0.15,
	}
	return dto, nil
}

func (api *giftApiImpl) DoCreateOrder(ctx context.Context, dto *types.OrderDTO) (*sink.OrderCreateResp, error) {
	// 构造请求体
	var bizReq sink.BizDataJSON[sink.OrderCreateReq]
	err := json.Unmarshal([]byte(dto.DataJSON), &bizReq)
	if err != nil {
		return nil, err
	}
	pubCode := bizReq.Body.PublicCode
	if pubCode == "" {
		return nil, errors.New("publicCode is required")
	}
	baseCodes, err := api.pub.GetBaseCodesByPublicCode(pubCode)
	if err != nil {
		return nil, err
	}
	if len(baseCodes) == 0 {
		return nil, fmt.Errorf("publicCode=%s not found", pubCode)
	}
	// =============== 处理 baseCodes ===============
	// 这里有两种常见场景：
	// A) 只需要取第一个 baseCode 并拼到请求体
	// B) 需要对所有 baseCode 都分别发起HTTP请求,
	//    或者把它们合并到同一次请求体中？

	// 下面演示 A) “只取第一个”的简化逻辑
	bc := baseCodes[0]
	log.Printf("[DoCreateOrder] got baseCode=%s for publicCode=%s\n", bc, pubCode)

	// 3) 发起上游 HTTP 请求
	//    先把 bizReq.Body 改造成实际发包的数据，比如再插入 baseCode
	bizReqMap := map[string]interface{}{}
	// 先把 "bizReq.Body" marshal => map
	bodyBytes, _ := json.Marshal(bizReq.Body)
	json.Unmarshal(bodyBytes, &bizReqMap)

	// 增加/覆盖 baseCode 字段
	// 对于上游接口，
	// 由于同构关系. 所以当前pub的baseCode就是上游的publicCode
	bizReqMap["publicCode"] = bc
	// 注意!!!!! 通过这个接口发起的调用请求虽然会在各个节点产生订单
	// 但是这样的订单叫做#线上订单#，结算佣金永远是0
	bizReqMap["source"] = "VIP_GIFT"

	// 最终请求体
	reqBytes, _ := json.Marshal(bizReqMap)

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.upstreamURL["CreateOrder"], bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("[giftApi.DoCreateOrder] http.NewRequestWithContext error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream status=%d body=%s", resp.StatusCode, string(b))
	}

	var createResp sink.OrderCreateResp
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, err
	}
	return &createResp, nil
}
func (api *giftApiImpl) DoQueryOrder(ctx context.Context, ids []string) ([]sink.OrderQueryResp, error) {
	// 2) 根据解析到的信息，准备查询参数。这里假设 Fulu 的查询接口需要 productId、customerOrderNo 等
	//   - 你可以根据实际的第三方接口做调整
	queryParam := map[string][]string{
		"orderIds": ids,
	}
	reqBytes, err := json.Marshal(queryParam)
	if err != nil {
		return nil, fmt.Errorf("marshal queryParam fail: %w", err)
	}

	// 3) 发起 HTTP 请求到 Fulu 的查询接口(假设是 POST)
	url := api.upstreamURL["QueryOrder"] // 你需要替换为实际的查询地址
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create query httpReq fail: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := api.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fulu query request fail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fulu query fail: http code=%d", resp.StatusCode)
	}

	// 4) 解析响应到通用结构 CommonAPIResp
	var fuluResp sink.CommonListAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&fuluResp); err != nil {
		return nil, fmt.Errorf("decode fulu query resp fail: %w", err)
	}
	fmt.Printf("[DoQueryOrder] query upstream resp: %+v\n", fuluResp)

	// 5) 根据第三方返回的字段设置订单状态 / 数据
	//    - 假设fuluResp.Code == 200 时表示成功
	//    - fuluResp.Data 里包含订单状态/时间/等；可根据需要继续解析
	if fuluResp.Code != 200 {
		return nil, fmt.Errorf("fulu query result fail: code=%d, msg=%s", fuluResp.Code, fuluResp.Message)
	}

	return fuluResp.Data, nil
}
