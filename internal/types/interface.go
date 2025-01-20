// internal/types/interface.go
package types

import (
	"context"

	"10000hk.com/vip_gift/internal/sink"
)

// GiftSearchable focuses on ES indexing fields
type GiftSearchable interface {
	GetESID() string   // ES doc ID
	GetESName() string // name for searching
	GetESCategories() []string
	// optionally: GetESDescription(), etc.
}

// GiftBase 接口，描述最基础的权益品信息
type GiftBase interface {
	// 1. 唯一标识
	GetBaseCode() string

	// 2. 异步消息服务接口（可能需要多个url或者一种区分）
	GetCallbackURL() string
	GetQueryURL() string

	// 3. 上架/下架/其他状态
	GetStatus() int64
}

// Composition 接口，用于描述组合信息
type Composition interface {
	GetBaseCode() string
	GetSnapshot() string
	GetStrategy() string
}

// GiftPublic 接口，用于描述对外售卖的组合权益品
type GiftPublic interface {
	// 唯一标识
	GetPublicCode() string
	// 组合信息（可能包含一个或多个基础权益品）
	GetCompositions() []Composition
}

// --------------------- 新增的订单接口 ---------------------

// GiftOrder 用于描述一个“订单”所需的关键信息；
// 根据业务需要，可以只定义最核心的方法，也可额外扩展。
type GiftOrder interface {
	// 订单ID（可能用Snowflake生成）
	GetOrderId() string
	// 下游系统传入的订单ID
	GetDownstreamOrderId() string
	// 订单的JSON字段，如存放具体商品/权益数据
	GetDataJSON() string
	// 订单状态，1=待处理,2=已完成, etc.
	GetStatus() int64

	// 如果需要，可以继续加更多方法:
	// GetCreatedAt() time.Time
	// GetUpdatedAt() time.Time
}

type OrderApi interface {
	// DoSendSms - 用于向上游发送短信请求
	DoSendSms(ctx context.Context, req sink.SmsReq) (*sink.OrderCreateResp, error)
	ToOrderDto(ctx context.Context, req sink.OrderCreateReq) (OrderDTO, error)
	// DoCreateOrder - 用于向上游发送“创建订单”请求
	// 返回包含对方系统订单ID、状态、信息等(根据上游API响应而定)
	DoCreateOrder(ctx context.Context, dto *OrderDTO) (*sink.OrderCreateResp, error)
	DoQueryOrder(ctx context.Context, ids []string) ([]sink.OrderQueryResp, error)
}
