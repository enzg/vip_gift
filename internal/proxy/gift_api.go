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
	order       service.OrderService
	httpClient  *http.Client
}

func NewGiftApi(upstreamURL map[string]string, pubSvc service.PubService, orderSvc service.OrderService) types.OrderApi {
	return &giftApiImpl{
		upstreamURL: upstreamURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		pub:   pubSvc,
		order: orderSvc,
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

	bizReqMap := map[string]any{}

	if pubCode == "" {
		return nil, errors.New("publicCode is required")
	}
	productId, err := api.pub.GetGncOriginDataByPublicCode(pubCode)
	if err != nil {
		// 没有gncOriginData, 尝试原来的逻辑
		log.Printf("[DoCreateOrder] GetGncOriginDataByPublicCode error: %v\n", err)
		baseCodes, err := api.pub.GetBaseCodesByPublicCode(pubCode)
		if err != nil {
			return nil, err
		}
		if len(baseCodes) == 0 {
			log.Printf("[DoCreateOrder] no baseCode found for publicCode=%s\n", pubCode)
			return nil, fmt.Errorf("publicCode=%s not found", pubCode)
		}
		bizReqMap["publicCode"] = baseCodes[0]
		bizReqMap["source"] = "VIP_GIFT"
		log.Printf("[DoCreateOrder] got baseCode=%s for publicCode=%s\n", baseCodes[0], pubCode)
	} else {
		// 有gncOriginData, 直接用
		log.Printf("[DoCreateOrder] GetGncOriginDataByPublicCode success: %s\n", productId)
		bizReq.Body.ProductId = productId
		bizReq.Body.CustomerOrderNo = dto.DownstreamOrderId
		bizReqMap["publicCode"] = productId

		bizReqMap["source"] = "VIP_FULU"
	}
	log.Printf("[DoCreateOrder] bizReqMap: %+v\n", bizReqMap)
	bodyBytes, _ := json.Marshal(bizReq.Body)
	json.Unmarshal(bodyBytes, &bizReqMap)

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

	var orderResults []sink.OrderQueryResp
	for _, downloadOrderId := range ids {
		if downloadOrderId == "" {
			continue
		}
		resp, err := api.queryOrderByDownstreamOrderId(ctx, downloadOrderId)
		if err != nil {
			log.Printf("[DoQueryOrder] queryOrderByDownstreamOrderId error: %v\n", err)
			continue
		}
		order, err := api.order.GetOrderByDownstreamOrderId(ctx, downloadOrderId)
		if err != nil {
			log.Printf("[DoQueryOrder] GetOrderByDownstreamOrderId error, 订单没有找到. 跳过这个订单: %v\n", err)
			continue
		}
		resp.OrderId = order.OrderId
		orderResults = append(orderResults, *resp)
	}
	return orderResults, nil
}

// queryOrderByDownstreamOrderId 查询订单
func (api *giftApiImpl) queryOrderByDownstreamOrderId(ctx context.Context, id string) (*sink.OrderQueryResp, error) {

	order, err := api.order.GetOrderByDownstreamOrderId(ctx, id)
	if err != nil {
		return nil, err
	}

	queryParam := map[string]any{
		"orderId": id,
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

	// // 4) 解析响应到通用结构 CommonAPIResp
	// var fuluResp sink.CommonAPIResp
	// if err := json.NewDecoder(resp.Body).Decode(&fuluResp); err != nil {
	// 	return nil, fmt.Errorf("decode fulu query resp fail: %w", err)
	// }

	// 假设我们已经调用了外部接口，并把响应 JSON 反序列化到 fuluResp
	var fuluResp sink.CommonAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&fuluResp); err != nil {
		return nil, fmt.Errorf("decode fulu query resp fail: %w", err)
	}
	fmt.Printf("[fulu_api_DoQuery] fuluResp: %+v", fuluResp)

	// 1. 将 fuluResp.Data 断言成 map[string]interface{}
	dataMap, ok := fuluResp.Data.(map[string]interface{})
	if !ok {
		log.Printf("[fulu_api_DoQuery] fuluResp.Data is not a map, got %T", fuluResp.Data)
		dataMap = make(map[string]interface{})
		dataMap["orderStatus"] = 0 // 默认状态
	}

	//  2. 取出 orderStatus 字段
	//     这里要先看外部返回的是数字(例如 40)还是字符串("40" / "订购失败")。
	//     * 如果是数字:   dataMap["orderStatus"].(float64)
	//     * 如果是字符串: dataMap["orderStatus"].(string)
	//
	//     假设接口返回的是数字，比如 40 表示“订购失败”：
	rawStatus, exists := dataMap["orderStatus"]
	if !exists {
		return nil, fmt.Errorf("orderStatus not found in fuluResp.Data")
	}

	// 3. 将其转换为 float64，再转成 int 后赋给 FuluOrderStatus
	statusFloat, ok := rawStatus.(float64)
	if !ok {
		return nil, fmt.Errorf("orderStatus is not a number, got %T", rawStatus)
	}

	// 4. 转换为我们定义的枚举类型
	fuluStatus := FuluOrderStatus(int64(statusFloat))

	// 5) 根据第三方返回的字段设置订单状态 / 数据
	//    - 假设fuluResp.Code == 200 时表示成功
	//    - fuluResp.Data 里包含订单状态/时间/等；可根据需要继续解析
	if fuluResp.Code != 200 {
		return nil, fmt.Errorf("fulu query result fail: code=%d, msg=%s", fuluResp.Code, fuluResp.Message)
	}

	// 5. 将 fuluResp.Data 转换为 JSON 字符串
	dataJSON, err := json.Marshal(fuluResp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal fuluResp.Data fail: %w", err)
	}

	// 准备返回给上层的查询结果（这里仅示范把整个 data 原样塞进去）
	return &sink.OrderQueryResp{
		OrderId:           order.OrderId,
		DownstreamOrderId: id,
		DataJSON:          string(dataJSON),
		Status:            int64(fuluStatus.ToOrderStatus()),
		StatusText:        fuluStatus.ToOrderStatus().String(),
		Remark:            fuluStatus.ToOrderStatus().Remark(),
	}, nil
}

type FuluOrderStatus int64

const (
	FuluOrderStatusWaitOrder    FuluOrderStatus = 10 // 待订购
	FuluOrderStatusOrdering     FuluOrderStatus = 20 // 订购中
	FuluOrderStatusOrderSuccess FuluOrderStatus = 30 // 订购成功
	FuluOrderStatusOrderFail    FuluOrderStatus = 40 // 订购失败
	FuluOrderStatusSuspicious   FuluOrderStatus = 50 // 订单可疑
)

func (f FuluOrderStatus) String() string {
	switch f {
	case FuluOrderStatusWaitOrder:
		return "待订购"
	case FuluOrderStatusOrdering:
		return "订购中"
	case FuluOrderStatusOrderSuccess:
		return "订购成功"
	case FuluOrderStatusOrderFail:
		return "订购失败"
	case FuluOrderStatusSuspicious:
		return "订单可疑"
	default:
		return fmt.Sprintf("未知状态(%d)", int(f))
	}
}

// 可选：让 JSON 中存储/解析成中文字符串
func (f FuluOrderStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.String())
}

func (f *FuluOrderStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "待订购":
		*f = FuluOrderStatusWaitOrder
	case "订购中":
		*f = FuluOrderStatusOrdering
	case "订购成功":
		*f = FuluOrderStatusOrderSuccess
	case "订购失败":
		*f = FuluOrderStatusOrderFail
	case "订单可疑":
		*f = FuluOrderStatusSuspicious
	default:
		return errors.New("未知 FuluOrderStatus: " + str)
	}
	return nil
}

// ToOrderStatus 将 FuluOrderStatus 转成 OrderStatus
func (f FuluOrderStatus) ToOrderStatus() types.OrderStatus {
	switch f {
	case FuluOrderStatusWaitOrder:
		return types.StatusInit // 10 -> 0 (init)
	case FuluOrderStatusOrdering:
		return types.StatusPending // 20 -> 100 (init)，也可以考虑单独状态
	case FuluOrderStatusOrderSuccess:
		return types.StatusSuccess // 30 -> 200
	case FuluOrderStatusOrderFail:
		return types.StatusUpstreamFail // 40 -> 500
	case FuluOrderStatusSuspicious:
		// 根据需求，如果可疑订单也算失败，可映射下游失败
		// 或者你要再单独定义一个 OrderStatus = 600 "Suspicious" 也可以
		return types.StatusDownstreamFail // 50 -> 400 (示例)
	default:
		// 未知状态时，可考虑给默认值或报错，这里先给 init
		return types.StatusInit
	}
}
