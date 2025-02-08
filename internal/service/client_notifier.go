package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"10000hk.com/vip_gift/internal/types"
)

// UpstreamNotifier 封装了「通知上游系统」的行为
type UpstreamNotifier interface {
	NotifyOrderStatus(ctx context.Context, orderDTO *types.OrderDTO) error
}

type upstreamNotifier struct {
	httpClient *http.Client
	notifyURL  string
}

// NewUpstreamNotifier 创建一个 UpstreamNotifier 的默认实现
func NewUpstreamNotifier(notifyURL string) UpstreamNotifier {
	return &upstreamNotifier{
		httpClient: &http.Client{
			Timeout: 5 * time.Second, // 可以视情况调大
		},
		notifyURL: notifyURL,
	}
}

// NotifyOrderStatus 发送 HTTP POST 到上游接口
func (u *upstreamNotifier) NotifyOrderStatus(ctx context.Context, orderDTO *types.OrderDTO) error {
	payload := map[string]interface{}{
		"upstreamOrderSn": orderDTO.OrderId, // 也可能是 orderDTO.DownstreamOrderId, 视具体需求
		"message":         orderDTO.Remark,
		"status":          int64(orderDTO.Status),
		"statusText":      orderDTO.Status.String(),
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.notifyURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http do error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify upstream failed: status=%d", resp.StatusCode)
	}

	// 你也可以读取 body 并检查具体返回
	// bodyBytes, _ := io.ReadAll(resp.Body)
	// log.Printf("Upstream response: %s", string(bodyBytes))

	return nil
}
