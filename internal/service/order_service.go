// internal/service/order_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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
	GetOrderEntity(ctx context.Context, orderId string) (*types.OrderEntity, error)
	UpdateOrder(ctx context.Context, ent *types.OrderEntity) error
	GetOrderByDownstreamOrderId(ctx context.Context, downstreamOrderId string) (*types.OrderEntity, error)

	// ListOrder 分页获取订单列表
	// ListOrder(ctx context.Context, page, size int64) ([]types.OrderDTO, int64, error)
	ListOrder(ctx context.Context, page, size int64, orderIds, downstreamIds []string) ([]types.OrderDTO, int64, error)
	// PublishOrderUpdate 发送订单更新消息
	PublishOrderUpdate(ctx context.Context, downstreamOrderId string, message []byte) error
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
		return nil, fmt.Errorf("[vip order-service] downstreamOrderId is required")
		// generatedDsId := fmt.Sprintf("DS-%d", generateRandom()) // 你可以用 Snowflake 等更好的生成
		// dto.DownstreamOrderId = generatedDsId
		// log.Printf("[CreateOrder] No downstreamOrderId provided, generated one: %s\n", generatedDsId)
	}
	if dto.DataJSON == "" {
		// return nil, fmt.Errorf("dataJSON is required")
		dto.DataJSON = "{}"
	}

	// 生成 orderId (snowflake)
	var orderId string
	if s.snowflakeFn != nil {
		orderId = s.snowflakeFn()
	} else {
		orderId = fmt.Sprintf("VIP-%d", generateRandom())
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
		Topic: "test-vip-order-create",
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
		return fmt.Errorf("StoreToDB: orderId is required")
	}

	// 先到 Repo 查一下
	existing, err := s.repo.GetOrderByOrderId(dto.OrderId)
	// 注意，目前 GetOrderByOrderId 如果找不到，会返回自定义错误 "订单不存在, orderId=xxx"
	// 你也可以让它原样返回 gorm.ErrRecordNotFound，以便用 errors.Is 来判断。

	if err != nil {
		// 若是“订单不存在”类的错误 => 说明尚无记录 => 执行插入
		if err.Error() != "" &&
			(strings.Contains(err.Error(), "订单不存在") /* 或者 errors.Is(err, gorm.ErrRecordNotFound) */) {
			// 插入
			newEnt := &types.OrderEntity{
				OrderId:           dto.OrderId,           // 初次创建时可使用
				DownstreamOrderId: dto.DownstreamOrderId, // 初次创建时可使用
				DataJSON:          dto.DataJSON,
				Status:            dto.Status,
				Remark:            dto.Remark,
				UserSn:            dto.UserSn,
				ParentSn:          dto.ParentSn,
				CommissionRule:    dto.CommissionRule,
				CommissionSelf:    dto.CommissionSelf,
				CommissionParent:  dto.CommissionParent,
				Channel:           dto.Channel,
			}
			if errC := s.repo.CreateOrder(newEnt); errC != nil {
				return fmt.Errorf("StoreToDB: create error: %w", errC)
			}
			log.Printf("[StoreToDB] new order %s inserted.\n", dto.OrderId)
			return nil
		}
		// 如果是其它错误，就直接返回
		return fmt.Errorf("StoreToDB: unexpected err in GetOrderByOrderId: %w", err)
	}

	// ================
	// 已存在 => 更新
	// ================

	// 只更新可变的字段( Status / Remark 等)
	existing.Status = dto.Status
	existing.Remark = dto.Remark

	if errU := s.repo.UpdateOrder(existing); errU != nil {
		return fmt.Errorf("StoreToDB: update error: %w", errU)
	}
	log.Printf("[StoreToDB] order %s updated in DB\n", dto.OrderId)

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
		Remark:            ent.Remark,
		UserSn:            ent.UserSn,
		ParentSn:          ent.ParentSn,
		CommissionRule:    ent.CommissionRule,
		CommissionSelf:    ent.CommissionSelf,
		CommissionParent:  ent.CommissionParent,
		TradeStatus:       ent.TradeStatus,
		RefundStatus:      ent.RefundStatus,
		DeliveryStatus:    ent.DeliveryStatus,
		SettlementStatus:  ent.SettlementStatus,
	}
	return dto, nil
}

// GetOrderEntity 通过 orderId 查询订单实体
func (s *orderServiceImpl) GetOrderEntity(ctx context.Context, orderId string) (*types.OrderEntity, error) {
	return s.repo.GetOrderByOrderId(orderId)
}

// UpdateOrder 更新订单，仅更新非空字段
func (s *orderServiceImpl) UpdateOrder(ctx context.Context, ent *types.OrderEntity) error {
	updateData := map[string]any{}

	if ent.TradeStatus != "" {
		updateData["trade_status"] = ent.TradeStatus
	}
	if ent.RefundStatus != "" {
		updateData["refund_status"] = ent.RefundStatus
	}
	if ent.DeliveryStatus > 0 {
		updateData["delivery_status"] = ent.DeliveryStatus
	}
	if ent.SettlementStatus > 0 {
		updateData["settlement_status"] = ent.SettlementStatus
	}

	if len(updateData) == 0 {
		return nil // 没有需要更新的字段
	}

	return s.repo.UpdateOrder(ent)
}
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
			UserSn:            e.UserSn,
			ParentSn:          e.ParentSn,
			CommissionRule:    e.CommissionRule,
			CommissionSelf:    e.CommissionSelf,
			CommissionParent:  e.CommissionParent,
			TradeStatus:       e.TradeStatus,
			RefundStatus:      e.RefundStatus,
			DeliveryStatus:    e.DeliveryStatus,
			SettlementStatus:  e.SettlementStatus,
		}
	}
	return dtos, total, nil
}

// OrderService 中新增的方法
func (s *orderServiceImpl) GetOrderByDownstreamOrderId(ctx context.Context, downstreamOrderId string) (*types.OrderEntity, error) {
	var order *types.OrderEntity
	order, err := s.repo.GetOrderByDownstreamOrderId(downstreamOrderId)
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}
	return order, nil
}

// 如需 ES,可加 indexToES, etc
func generateRandom() int64 {
	return 100000 + time.Now().UnixNano()%100000
}
func (s *orderServiceImpl) PublishOrderUpdate(ctx context.Context, downstreamOrderId string, message []byte) error {
	return s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(downstreamOrderId),
		Value: message,
		Topic: "test-vip-order-update",
	})
}
