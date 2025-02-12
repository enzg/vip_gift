package mq

import (
	"context"
	"fmt"
	"log"
	"time"

	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/internal/types"
)

// QueryTask represents a request to query an order after a delay
type QueryTask struct {
	OrderDTO *types.OrderDTO // Information about the order
	Delay    time.Duration   // How long to wait before querying
	OrderApi types.OrderApi  // The API to call for DoQueryOrder
	OrderSvc service.OrderService
}

// QueryScheduler runs in the background, processing scheduled queries.
type QueryScheduler struct {
	tasksChan chan QueryTask
	stopChan  chan struct{}
	notifier  service.UpstreamNotifier // <--- 新增
}

// NewQueryScheduler creates a QueryScheduler with a buffered channel
func NewQueryScheduler(bufferSize int, notifier service.UpstreamNotifier) *QueryScheduler {
	return &QueryScheduler{
		tasksChan: make(chan QueryTask, bufferSize),
		stopChan:  make(chan struct{}),
		notifier:  notifier,
	}
}

// Start launches the background loop
func (qs *QueryScheduler) Start() {
	go qs.loop()
}

// Stop signals the scheduler to stop
func (qs *QueryScheduler) Stop() {
	close(qs.stopChan)
	close(qs.tasksChan)
}

// loop waits for incoming tasks and schedules them
func (qs *QueryScheduler) loop() {
	for {
		select {
		case <-qs.stopChan:
			log.Println("[QueryScheduler] stopping scheduler loop")
			return
		case task, ok := <-qs.tasksChan:
			if !ok {
				return
			}
			// For each incoming task, launch a separate goroutine that
			// waits the specified delay and then calls DoQueryOrder
			go qs.handleTask(task)
		}
	}
}

// handleTask sleeps for 'Delay' then calls DoQueryOrder
func (qs *QueryScheduler) handleTask(task QueryTask) {
	timer := time.NewTimer(task.Delay)
	defer timer.Stop()

	<-timer.C // Wait for the delay

	// Attempt the query
	// For demonstration, we pass a slice of 1 ID.
	fmt.Printf("[QueryScheduler] querying order=%s\n", task.OrderDTO.DownstreamOrderId)
	orderIds := []string{task.OrderDTO.DownstreamOrderId}
	resp, err := task.OrderApi.DoQueryOrder(context.Background(), orderIds)
	if err != nil {
		// handle error: update DB to reflect error status or log it
		log.Printf("[QueryScheduler] DoQueryOrder error: %v\n", err)
		task.OrderDTO.Status = 500
		task.OrderDTO.Remark = fmt.Sprintf("query error: %v", err)
		_ = task.OrderSvc.StoreToDB(context.Background(), task.OrderDTO)
		return
	}

	// If success, parse 'resp' to see if there's a new status
	// For example, suppose sink.OrderQueryResp has a field `Status`
	// We'll just demonstrate with the first item:
	if len(resp) > 0 {
		// Suppose each OrderQueryResp has something like `OrderStatus`
		// you might map them to your internal Status. For now let's assume:
		//   2 -> success, 3 -> failed, etc.
		// We'll only use the first item to demonstrate:
		newStatus := resp[0].Status
		parsed, err := types.ConvertStringToOrderStatus(newStatus)
		if err != nil {
			// 处理未知枚举值，比如记录日志或给个默认值
		}
		task.OrderDTO.Status = parsed
		task.OrderDTO.Remark = parsed.Remark()
		_ = task.OrderSvc.StoreToDB(context.Background(), task.OrderDTO)
		log.Printf("[QueryScheduler] updated order=%s to status=%d\n",
			task.OrderDTO.OrderId, task.OrderDTO.Status)
		if err := qs.notifier.NotifyOrderStatus(context.Background(), task.OrderDTO); err != nil {
			// 可以根据实际需求重试或忽略
			log.Printf("[QueryScheduler] notify upstream error: %v\n", err)
		}
	}
}

// ScheduleQuery enqueues a QueryTask
func (qs *QueryScheduler) ScheduleQuery(task QueryTask) {
	qs.tasksChan <- task
}
