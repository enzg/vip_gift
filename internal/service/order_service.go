// internal/service/order_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/types"
)

// OrderService 定义订单接口
type OrderService interface {
	// CreateOrder 仅发送消息到 Kafka, 不写DB
	CreateOrder(ctx context.Context, dto *types.OrderDTO) (*types.OrderDTO, error)

	// StoreToDB 真正插入数据库(消费者侧调用)
	StoreToDB(ctx context.Context, dto *types.OrderDTO) error

	// GetOrder 根据orderId查询订单
	GetOrder(ctx context.Context, orderId string) (*types.OrderDTO, error)

	// ListOrder 分页获取订单列表
	// ListOrder(ctx context.Context, page, size int64) ([]types.OrderDTO, int64, error)
	ListOrder(ctx context.Context, page, size int64, orderIds, downstreamIds []string) ([]types.OrderDTO, int64, error)
}

// orderServiceImpl
type orderServiceImpl struct {
	repo        repository.OrderRepo
	kafkaWriter *kafka.Writer
	snowflakeFn func() string
	// esClient   *elasticsearch.Client (如需写ES可加)
}

var _ OrderService = (*orderServiceImpl)(nil)

// NewOrderService
func NewOrderService(repo repository.OrderRepo, kWriter *kafka.Writer, sfFn func() string) OrderService {
	return &orderServiceImpl{
		repo:        repo,
		kafkaWriter: kWriter,
		snowflakeFn: sfFn,
	}
}

// -------------------------------------------------------------------
// 1) CreateOrder: 不写DB, 只发 Kafka
// -------------------------------------------------------------------
func (s *orderServiceImpl) CreateOrder(ctx context.Context, dto *types.OrderDTO) (*types.OrderDTO, error) {
	if dto.DownstreamOrderId == "" {
		return nil, fmt.Errorf("downstreamOrderId is required")
	}
	if dto.DataJSON == "" {
		return nil, fmt.Errorf("dataJSON is required")
	}

	// 生成 orderId (snowflake)
	var orderId string
	if s.snowflakeFn != nil {
		orderId = s.snowflakeFn()
	} else {
		orderId = fmt.Sprintf("ORD-%d", generateRandom())
	}
	dto.OrderId = orderId

	// 只发Kafka (本模式)
	if s.kafkaWriter == nil {
		return nil, fmt.Errorf("kafkaWriter is nil, can't produce message")
	}
	msgBytes, _ := json.Marshal(dto)
	err := s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(orderId),
		Value: msgBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka produce error: %v", err)
	}
	log.Printf("[CreateOrder] order %s produced to Kafka (no DB write)\n", orderId)

	return dto, nil
}

// -------------------------------------------------------------------
// 2) StoreToDB: 真正写数据库 (消费者调用)
// -------------------------------------------------------------------
func (s *orderServiceImpl) StoreToDB(ctx context.Context, dto *types.OrderDTO) error {
	if dto.OrderId == "" {
		return fmt.Errorf("orderId is required for StoreToDB")
	}
	ent := &types.OrderEntity{
		OrderId:           dto.OrderId,
		DownstreamOrderId: dto.DownstreamOrderId,
		DataJSON:          dto.DataJSON,
		Status:            dto.Status,
	}
	if err := s.repo.CreateOrder(ent); err != nil {
		return fmt.Errorf("StoreToDB db error: %w", err)
	}
	log.Printf("[StoreToDB] order %s inserted into DB\n", dto.OrderId)
	return nil
}

// -------------------------------------------------------------------
// 3) GetOrder: 根据orderId查询订单
// -------------------------------------------------------------------
func (s *orderServiceImpl) GetOrder(ctx context.Context, orderId string) (*types.OrderDTO, error) {
	ent, err := s.repo.GetOrderByOrderId(orderId)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("订单不存在: %s", orderId)
		}
		return nil, err
	}
	dto := &types.OrderDTO{
		OrderId:           ent.OrderId,
		DownstreamOrderId: ent.DownstreamOrderId,
		DataJSON:          ent.DataJSON,
		Status:            ent.Status,
	}
	return dto, nil
}

// -------------------------------------------------------------------
// 4) ListOrder: 分页获取订单列表
// -------------------------------------------------------------------
// func (s *orderServiceImpl) ListOrder(ctx context.Context, page, size int64) ([]types.OrderDTO, int64, error) {
// 	ents, total, err := s.repo.ListOrder(page, size)
// 	if err != nil {
// 		return nil, 0, err
// 	}
// 	dtos := make([]types.OrderDTO, len(ents))
// 	for i, e := range ents {
// 		dtos[i] = types.OrderDTO{
// 			OrderId:           e.OrderId,
// 			DownstreamOrderId: e.DownstreamOrderId,
// 			DataJSON:          e.DataJSON,
// 			Status:            e.Status,
// 		}
// 	}
// 	return dtos, total, nil
// }
func (s *orderServiceImpl) ListOrder(ctx context.Context, page, size int64, orderIds, downstreamIds []string) ([]types.OrderDTO, int64, error) {
	ents, total, err := s.repo.ListOrder(page, size, orderIds, downstreamIds)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]types.OrderDTO, len(ents))
	for i, e := range ents {
		dtos[i] = types.OrderDTO{
			OrderId:           e.OrderId,
			DownstreamOrderId: e.DownstreamOrderId,
			DataJSON:          e.DataJSON,
			Status:            e.Status,
		}
	}
	return dtos, total, nil
}
// 如需 ES,可加 indexToES, etc
func generateRandom() int64 {
	return 100000 // demo
}
