// internal/sink/dto.go
package sink

// OrderCreateReq - 前端请求体
type OrderCreateReq struct {
	Phone             string `json:"phone"`                // 必填/可选？
	DownstreamOrderId string `json:"downstreamOrderId"`    //
	PublicCode        string `json:"publicCode,omitempty"` // 产品编码
	ProductId         string `json:"productId,omitempty"`  // 产品ID
	Otac              string `json:"otac,omitempty"`       // 短信验证码
	DataJSON          string `json:"dataJSON,omitempty"`   // 可选，存各种其他信息
	Amount            int64  `json:"amount,omitempty"`     // 可选，金额
	Source            string `json:"source,omitempty"`     // 可选，订单来源
	PartnerId         string `json:"partnerId,omitempty"`  // 可选，合作方ID
	ParentSn          string `json:"parentSn,omitempty"`   // 可选，上级编号
}

// OrderCreateResp - 返回给前端
type OrderCreateResp struct {
	OrderId string `json:"orderId"`
	Status  int64  `json:"status"`
	Message string `json:"message,omitempty"`
}
type OrderChargeReq struct {
	Phone     string `json:"phone"`
	ProductId string `json:"productId"`
	Amount    int64  `json:"amount"`
}
type BizDataJSON[T any] struct {
	Body  T      `json:"body"`
	Extra string `json:"extra,omitempty"`
}
type SmsReq struct {
	PublicCode string `json:"publicCode"`
	Phone      string `json:"phone"`
}

type OrderQueryResp struct {
	OrderId           string `json:"orderId"`
	DownstreamOrderId string `json:"downstreamOrderId"`
	DataJSON          string `json:"dataJSON"`
	Status            string `json:"status"`
}
type CommonAPIResp struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type CommonListAPIResp struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []OrderQueryResp `json:"data,omitempty"`
}
