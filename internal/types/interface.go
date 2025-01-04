// internal/types/interface.go
package types

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
