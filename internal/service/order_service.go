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

	GetOrder(ctx context.Context, orderId string) (*types.OrderDTO, error)
	// 也可再加 UpdateOrder / DeleteOrder / ListOrder
}

// orderServiceImpl
type orderServiceImpl struct {
	repo        repository.OrderRepo
	kafkaWriter *kafka.Writer
	snowflakeFn func() string
	// esClient   *elasticsearch.Client
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

// 1) CreateOrder: 不写DB, 只发 Kafka
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

	// 只发Kafka
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

	// 返回DTO
	return dto, nil
}

// 2) StoreToDB: 真正写数据库 (消费者调用)
func (s *orderServiceImpl) StoreToDB(ctx context.Context, dto *types.OrderDTO) error {
	if dto.OrderId == "" {
		return fmt.Errorf("orderId is required for StoreToDB")
	}
	ent := &types.OrderEntity{
		OrderId:           dto.OrderId,
		DownstreamOrderId: dto.DownstreamOrderId,
		DataJSON:          dto.DataJSON,
		Status:            dto.Status, // or 1= default
	}
	if err := s.repo.CreateOrder(ent); err != nil {
		return fmt.Errorf("StoreToDB db error: %w", err)
	}
	log.Printf("[StoreToDB] order %s inserted into DB\n", dto.OrderId)
	return nil
}

// 3) GetOrder
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

// 如果需要 ES 或更多方法,可扩展
func generateRandom() int64 {
	return 100000 // demo
}
