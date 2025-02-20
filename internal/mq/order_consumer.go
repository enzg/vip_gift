package mq

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"
)

// OrderMessage 用于解析 `vip-order-create` 主题的 Kafka 消息
type OrderMessage struct {
	DownstreamOrderId string            `json:"downstreamOrderId"`
	DataJSON          string            `json:"dataJSON"`
	OrderId           string            `json:"orderId"`
	Status            types.OrderStatus `json:"status"`
	CommissionSelf    float64           `json:"commissionSelf"`
	CommissionParent  float64           `json:"commissionParent"`
	UserSn            string            `json:"userSn"`
	ParentSn          string            `json:"parentSn"`
}

// OrderUpdateMessage 用于解析 `order-update` 主题的 Kafka 消息
type OrderUpdateMessage struct {
	OrderId           string `json:"orderId,omitempty"`
	DownstreamOrderId string `json:"downstreamOrderId,omitempty"`
	TradeStatus       string `json:"tradeStatus,omitempty"`
	RefundStatus      string `json:"refundStatus,omitempty"`
	DeliveryStatus    int64  `json:"deliveryStatus,omitempty"`
	SettlementStatus  int64  `json:"settlementStatus,omitempty"`
}

// OrderConsumer 结构
type OrderConsumer struct {
	reader         *kafka.Reader
	updateReader   *kafka.Reader
	stopCh         chan struct{}
	orderService   service.OrderService
	pub            service.PubService
	queryScheduler *QueryScheduler
}

// NewOrderConsumer 初始化消费者（支持 `vip-order-create` 和 `order-update`）
func NewOrderConsumer(brokers []string, createTopic, updateTopic, groupID string, orderSvc service.OrderService, pubSvc service.PubService, qs *QueryScheduler) *OrderConsumer {
	createReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    createTopic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
	})

	updateReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    updateTopic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
	})

	return &OrderConsumer{
		reader:         createReader,
		updateReader:   updateReader,
		stopCh:         make(chan struct{}),
		orderService:   orderSvc,
		pub:            pubSvc,
		queryScheduler: qs,
	}
}

// Start 启动两个 Kafka 消费者
func (o *OrderConsumer) Start() {
	go o.runCreateConsumer() // 处理订单创建
	go o.runUpdateConsumer() // 处理订单状态更新
}

// Stop 关闭 Kafka 消费者
func (o *OrderConsumer) Stop() {
	close(o.stopCh)
	_ = o.reader.Close()
	_ = o.updateReader.Close()
}

// ========== 1. 处理 `vip-order-create`（创建订单） ==========

func (o *OrderConsumer) runCreateConsumer() {
	log.Println("[OrderConsumer] Starting vip-order-create consumer loop")
	defer log.Println("[OrderConsumer] Stopped vip-order-create consumer loop")

	for {
		select {
		case <-o.stopCh:
			return
		default:
		}

		m, err := o.reader.FetchMessage(context.Background())
		if err != nil {
			log.Printf("[OrderConsumer] Fetch message error: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		var msg OrderMessage
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			log.Printf("[OrderConsumer] Unmarshal error: %v\n", err)
			_ = o.reader.CommitMessages(context.Background(), m)
			continue
		}

		o.handleCreateOrder(msg)

		if err := o.reader.CommitMessages(context.Background(), m); err != nil {
			log.Printf("[OrderConsumer] Commit error: %v\n", err)
		}
	}
}

// 处理订单创建
func (o *OrderConsumer) handleCreateOrder(msg OrderMessage) {
	log.Printf("[OrderConsumer] Processing order: orderId=%s downstreamId=%s status=%d\n",
		msg.OrderId, msg.DownstreamOrderId, msg.Status)

	dto := &types.OrderDTO{
		OrderId:           msg.OrderId,
		DownstreamOrderId: msg.DownstreamOrderId,
		DataJSON:          msg.DataJSON,
		Status:            msg.Status,
		UserSn:            msg.UserSn,
		ParentSn:          msg.ParentSn,
		CommissionRule:    "MF",
		CommissionSelf:    msg.CommissionSelf,
		CommissionParent:  msg.CommissionParent,
	}

	if err := o.orderService.StoreToDB(context.Background(), dto); err != nil {
		log.Printf("[OrderConsumer] Store to DB error: %v\n", err)
		return
	}

	o.scheduleQueryAttempts(dto)
}

// ========== 2. 处理 `order-update`（更新订单状态） ==========

func (o *OrderConsumer) runUpdateConsumer() {
	log.Println("[OrderConsumer] Starting order-update consumer loop")
	defer log.Println("[OrderConsumer] Stopped order-update consumer loop")

	for {
		select {
		case <-o.stopCh:
			return
		default:
		}

		m, err := o.updateReader.FetchMessage(context.Background())
		if err != nil {
			log.Printf("[OrderConsumer] Fetch order-update message error: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		var msg OrderUpdateMessage
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			log.Printf("[OrderConsumer] Unmarshal error: %v\n", err)
			_ = o.updateReader.CommitMessages(context.Background(), m)
			continue
		}

		o.handleUpdateOrder(msg)

		if err := o.updateReader.CommitMessages(context.Background(), m); err != nil {
			log.Printf("[OrderConsumer] Commit error: %v\n", err)
		}
	}
}

// 处理订单更新
func (o *OrderConsumer) handleUpdateOrder(msg OrderUpdateMessage) {
	var order *types.OrderEntity
	var err error

	if msg.DownstreamOrderId != "" {
		order, err = o.orderService.GetOrderByDownstreamOrderId(context.Background(), msg.DownstreamOrderId)
	}

	if err != nil {
		log.Printf("[OrderConsumer] Order not found: orderId=%s, downstreamOrderId=%s\n", msg.OrderId, msg.DownstreamOrderId)
		return
	}

	if msg.TradeStatus != "" {
		order.TradeStatus = msg.TradeStatus
	}
	if msg.RefundStatus != "" {
		order.RefundStatus = msg.RefundStatus
	}
	if msg.DeliveryStatus > 0 {
		order.DeliveryStatus = msg.DeliveryStatus
	}
	if msg.SettlementStatus > 0 {
		order.SettlementStatus = msg.SettlementStatus
	}

	if err := o.orderService.UpdateOrder(context.Background(), order); err != nil {
		log.Printf("[OrderConsumer] Failed to update order: %v\n", err)
	}
}

// ========== 3. 订单查询调度 ==========

func (o *OrderConsumer) scheduleQueryAttempts(dto *types.OrderDTO) {
	delays := []time.Duration{3 * time.Second, 7 * time.Second, 13 * time.Second, 31 * time.Second}
	for _, d := range delays {
		o.queryScheduler.ScheduleQuery(QueryTask{OrderDTO: dto, Delay: d, OrderSvc: o.orderService})
	}
	log.Printf("[OrderConsumer] Scheduled queries for order=%s\n", dto.OrderId)
}
