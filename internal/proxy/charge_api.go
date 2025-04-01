package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"10000hk.com/vip_gift/internal/sink"
	"10000hk.com/vip_gift/internal/types"
)

type chargeApiImpl struct {
	upstreamURL map[string]string
	httpClient  *http.Client
}

func NewChargeApi(upstreamURL map[string]string) types.OrderApi {
	return &chargeApiImpl{
		upstreamURL: upstreamURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}
func (api *chargeApiImpl) DoSendSms(ctx context.Context, req sink.SmsReq) (*sink.OrderCreateResp, error) {
	return &sink.OrderCreateResp{}, nil
}
func (api *chargeApiImpl) ToOrderDto(ctx context.Context, req sink.OrderCreateReq) (types.OrderDTO, error) {
	var downstreamOrderId string = req.DownstreamOrderId
	if downstreamOrderId == "" {
		return types.OrderDTO{}, nil
	}
	packReq := sink.BizDataJSON[sink.OrderChargeReq]{
		Body: sink.OrderChargeReq{
			Phone:             req.Phone,
			DownstreamOrderId: downstreamOrderId,
			ProductId:         req.PublicCode,
			Amount:            req.Amount,
		},
		Extra: req.DataJSON,
	}
	bizReqJSON, _ := json.Marshal(packReq)

	// 根据 publicCode 查找产品，获取 CommissionMF
	var commissionMF float64 = 0.0
	productLookupURL := "https://gift.10000hk.com/api/charge/product/list"

	// 构造请求payload，假设查询条件为 productId（publicCode）
	searchPayload, err := json.Marshal(map[string]any{
		"productId": req.PublicCode,
	})
	if err != nil {
		// 若payload构造失败，则记录日志后继续，commissionMF默认为0
		fmt.Printf("[ToOrderDto] marshal search payload error: %v\n", err)
	} else {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, productLookupURL, bytes.NewBuffer(searchPayload))
		if err == nil {
			httpReq.Header.Set("Content-Type", "application/json")
			resp, err := api.httpClient.Do(httpReq)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					// 定义返回数据结构，增加 commissionValue 字段
					type DataItem struct {
						Platform        string `json:"platform"`
						Product         string `json:"product"`
						Range           string `json:"range"`
						SalePrice       string `json:"salePrice"`
						ProductId       string `json:"productId"`
						CommissionValue string `json:"commissionValue"`
					}
					type APIResponse struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
						Data    struct {
							DataList []DataItem `json:"dataList"`
							Total    int        `json:"total"`
						} `json:"data"`
					}
					var apiResp APIResponse
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						fmt.Printf("[ToOrderDto] read product lookup resp error: %v\n", err)
					} else if err = json.Unmarshal(body, &apiResp); err != nil {
						fmt.Printf("[ToOrderDto] unmarshal product lookup resp error: %v\n", err)
					} else if apiResp.Code == 200 {
						// 在返回结果中查找匹配的产品（以 productId 比较）
						for _, item := range apiResp.Data.DataList {
							if item.ProductId == req.PublicCode {
								// 将 CommissionValue 从 string 转换为 float64
								commissionValue, errConv := strconv.ParseFloat(item.CommissionValue, 64)
								if errConv != nil {
									fmt.Printf("[ToOrderDto] convert CommissionValue error: %v\n", errConv)
								} else {
									commissionMF = commissionValue
								}
								break
							}
						}
					} else {
						fmt.Printf("[ToOrderDto] product lookup response code not 200, code=%d, msg=%s\n", apiResp.Code, apiResp.Message)
					}
				} else {
					fmt.Printf("[ToOrderDto] product lookup http status: %d\n", resp.StatusCode)
				}
			} else {
				fmt.Printf("[ToOrderDto] product lookup request error: %v\n", err)
			}
		} else {
			fmt.Printf("[ToOrderDto] create product lookup request error: %v\n", err)
		}
	}

	// 组装订单 DTO，同时设置佣金比例
	dto := types.OrderDTO{
		DownstreamOrderId: downstreamOrderId,
		PublicCode:        req.PublicCode,
		DataJSON:          string(bizReqJSON),
		Status:            0,
		Remark:            "",
		CommissionRule:    "MF", // 权益业务通通默认秒返
		UserSn:            req.PartnerId,
		ParentSn:          req.ParentSn,
		// 根据查找到的 CommissionMF 计算下级/上级分佣
		CommissionSelf:   commissionMF * 0.80,
		CommissionParent: commissionMF * 0.20,
	}
	return dto, nil
}
func (api *chargeApiImpl) DoCreateOrder(ctx context.Context, dto *types.OrderDTO) (*sink.OrderCreateResp, error) {
	var bizReq sink.BizDataJSON[sink.OrderChargeReq]
	if err := json.Unmarshal([]byte(dto.DataJSON), &bizReq); err != nil {
		return nil, err
	}
	reqBytes, _ := json.Marshal(bizReq.Body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.upstreamURL["CreateOrder"], bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("chargeApi.DoCreateOrder: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chargeApi.DoCreateOrder: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream status=%d body=%s", resp.StatusCode, string(b))
	}
	var chargeResp sink.CommonAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&chargeResp); err != nil {
		return nil, fmt.Errorf("decode charge order resp fail: %w", err)
	}
	var status int64
	if chargeResp.Code != 200 {
		status = 500
	} else {
		status = 100
	}

	dataJsonBytes, err := json.Marshal(chargeResp.Data)
	if err != nil {
		dataJsonBytes = []byte("[chargeApi.DoCreateOrder] json.Marshal fail")
	}

	return &sink.OrderCreateResp{
		OrderId:    dto.OrderId,
		Status:     status,
		StatusText: types.OrderStatus(status).String(),
		Message:    string(dataJsonBytes),
	}, nil
}
func (api *chargeApiImpl) DoQueryOrder(ctx context.Context, ids []string) ([]sink.OrderQueryResp, error) {

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
		return nil, fmt.Errorf("charge query request fail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("charge query fail: http code=%d", resp.StatusCode)
	}

	// 4) 解析响应到通用结构 CommonAPIResp
	var chargeResp sink.CommonListAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&chargeResp); err != nil {
		return nil, fmt.Errorf("decode charge query resp fail: %w", err)
	}
	fmt.Printf("[DoQueryOrder] query upstream resp: %+v\n", chargeResp)

	// 5) 根据第三方返回的字段设置订单状态 / 数据
	if chargeResp.Code != 200 {
		return nil, fmt.Errorf("fulu query result fail: code=%d, msg=%s", chargeResp.Code, chargeResp.Message)
	}

	return chargeResp.Data, nil
}
