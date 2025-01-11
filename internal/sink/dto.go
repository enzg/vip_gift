// internal/sink/dto.go
package sink

// OrderCreateReq - 前端请求体
type OrderCreateReq struct {
	Phone             string `json:"phone"`                       // 必填/可选？
	PublicCode        string `json:"publicCode"`                  // 产品编码
	DownstreamOrderId string `json:"downstreamOrderId,omitempty"` // 可选 (不传则自动生成)
	Otac              string `json:"otac,omitempty"`              // 短信验证码
	DataJSON          string `json:"dataJSON,omitempty"`          // 可选，存各种其他信息
	Source            string `json:"source,omitempty"`            // 可选，订单来源
}

// OrderCreateResp - 返回给前端
type OrderCreateResp struct {
	OrderId string `json:"orderId"`
	Status  int64  `json:"status"`
	Message string `json:"message,omitempty"`
}

type BizDataJSON struct {
	Body  OrderCreateReq `json:"body"`
	Extra string         `json:"extra,omitempty"`
}
