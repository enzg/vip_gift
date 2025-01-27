package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
			Phone:     req.Phone,
			ProductId: req.ProductId,
			Amount:    req.Amount,
		},
		Extra: req.DataJSON,
	}
	bizReqJSON, _ := json.Marshal(packReq)
	dto := types.OrderDTO{
		DownstreamOrderId: downstreamOrderId,
		DataJSON:          string(bizReqJSON),
		Status:            0,
		Remark:            "",
		CommissionRule:    "MF", // 权益业务通通默认秒返
		UserSn:            req.PartnerId,
		ParentSn:          req.ParentSn,
	}
	return dto, nil
}
func (api *chargeApiImpl) DoCreateOrder(ctx context.Context, dto *types.OrderDTO) (*sink.OrderCreateResp, error) {
	var bizReq sink.BizDataJSON[sink.OrderChargeReq]
	if err := json.Unmarshal([]byte(dto.DataJSON), &bizReq); err != nil {
		return nil, err
	}
	reqBytes, _ := json.Marshal(bizReq.Body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.upstreamURL["createOrder"], bytes.NewReader(reqBytes))
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
		status = 1
	}

	dataJsonBytes, err := json.Marshal(chargeResp.Data)
	if err != nil {
		dataJsonBytes = []byte("[chargeApi.DoCreateOrder] json.Marshal fail")
	}

	return &sink.OrderCreateResp{
		OrderId: dto.OrderId,
		Status:  status,
		Message: string(dataJsonBytes),
	}, nil
}
func (api *chargeApiImpl) DoQueryOrder(ctx context.Context, ids []string) ([]sink.OrderQueryResp, error) {
	return make([]sink.OrderQueryResp, 0), nil
}
