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

type orderApiImpl struct {
	upstreamURL string
	pub         service.PubService
	httpClient  *http.Client
}

func NewOrderApi(upstreamURL string, pubSvc service.PubService) types.OrderApi {
	return &orderApiImpl{
		upstreamURL: upstreamURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		pub: pubSvc,
	}
}

func (api *orderApiImpl) DoSendSms(ctx context.Context, req sink.SmsReq) (*sink.OrderCreateResp, error) {
	// TODO: implement if needed
	return &sink.OrderCreateResp{}, nil
}
func (api *orderApiImpl) ToOrderDto(ctx context.Context, req sink.OrderCreateReq) (types.OrderDTO, error) {
	return types.OrderDTO{}, nil
}

func (api *orderApiImpl) DoCreateOrder(ctx context.Context, dto *types.OrderDTO) (*sink.OrderCreateResp, error) {
	// 构造请求体
	var bizReq sink.BizDataJSON
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.upstreamURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("[orderApi.DoCreateOrder] http.NewRequestWithContext error: %w", err)
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
