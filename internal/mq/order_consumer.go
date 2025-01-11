package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"
)

// OrderMessage 用于解析从kafka拉取的订单JSON
type OrderMessage struct {
	DownstreamOrderId string `json:"downstreamOrderId"`
	DataJSON          string `json:"dataJSON"`
	OrderId           string `json:"orderId"`
	Status            int64  `json:"status"`
}

// OrderConsumer 结构，包含 Reader
type OrderConsumer struct {
	reader       *kafka.Reader
	stopCh       chan struct{}
	orderService service.OrderService // 注入订单服务
	orderApi     types.OrderApi
}

// NewOrderConsumer 初始化消费者
func NewOrderConsumer(brokers []string, topic, groupID string, orderSvc service.OrderService, orderApi types.OrderApi) *OrderConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID, // 消费者组
		Topic:    topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		MaxWait:  1 * time.Second,
	})

	return &OrderConsumer{
		reader:       r,
		stopCh:       make(chan struct{}),
		orderService: orderSvc,
		orderApi:     orderApi,
	}
}

// Start 启动消费循环
func (o *OrderConsumer) Start() {
	go o.run()
}

// Stop 停止消费
func (o *OrderConsumer) Stop() {
	close(o.stopCh)
	_ = o.reader.Close()
}

func (o *OrderConsumer) run() {
	log.Println("[OrderConsumer] starting consume loop")
	defer log.Println("[OrderConsumer] consume loop stopped")

	for {
		select {
		case <-o.stopCh:
			return
		default:
		}

		// 拉取消息
		m, err := o.reader.FetchMessage(context.Background())
		if err != nil {
			log.Printf("[OrderConsumer] fetch message error: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// 解析消息
		var msg OrderMessage
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			log.Printf("[OrderConsumer] unmarshal error: %v\n", err)
			_ = o.reader.CommitMessages(context.Background(), m)
			continue
		}

		// 处理订单(真正写DB)
		o.handleOrder(msg)

		// 手动提交offset，避免重复消费
		if err := o.reader.CommitMessages(context.Background(), m); err != nil {
			log.Printf("[OrderConsumer] commit error: %v\n", err)
		}
	}
}

// 处理订单, 调用 orderService.StoreToDB
func (o *OrderConsumer) handleOrder(msg OrderMessage) {
	log.Printf("[OrderConsumer] got order: orderId=%s downstreamId=%s status=%d\n",
		msg.OrderId, msg.DownstreamOrderId, msg.Status)

	// 1) 转成 OrderDTO
	dto := &types.OrderDTO{
		OrderId:           msg.OrderId,
		DownstreamOrderId: msg.DownstreamOrderId,
		DataJSON:          msg.DataJSON,
		Status:            msg.Status,
		Remark:            "", // 可以根据需要设置. 创建时默认为空
	}

	// 2) 写DB
	if err := o.orderService.StoreToDB(context.Background(), dto); err != nil {
		log.Printf("[OrderConsumer] store to DB error: %v\n", err)
		return
	}
	// 3) 进一步逻辑: e.g. 通知, 回调, 更新状态...
	fmt.Printf("[OrderConsumer] order %s has been inserted into DB.\n", msg.OrderId)

	orderCreateResp, err := o.orderApi.DoCreateOrder(context.Background(), dto)
	if err != nil {
		log.Printf("[OrderConsumer] DoCreateOrder error: %v\n", err)
		dto.Status = 500
		dto.Remark = fmt.Sprintf("DoCreateOrder error: %v", err)
		_ = o.orderService.StoreToDB(context.Background(), dto)
		return
	}
	log.Printf("[OrderConsumer] DoCreateOrder resp: %+v\n", orderCreateResp)

}
