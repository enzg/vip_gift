// cmd/main.go
package main

import (
	"fmt"
	"log"

	"10000hk.com/vip_gift/config"
	"10000hk.com/vip_gift/internal/handler"
	"10000hk.com/vip_gift/internal/mq" // 新增: 引入消费者
	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/pkg"
)

var topicOrerCreate = "vip-order-create"
var kafkaUrl = "localhost:9092"
var consumerId = "order_consumer_group"

func main() {
	// 1) 加载环境变量
	config.LoadEnv()

	// 2) 初始化DB & ES
	db := config.InitDB()
	esClient := config.InitES()

	// 3) Fiber
	app := config.SetupFiber()

	// 4) 路由组： /api/product/gift
	api := app.Group("/api/product/gift")
	// 如果需要JWT保护 => api.Use(handler.JWTMiddleware(jwtSecretKey))

	// 5) 注册 Pub 模块
	pubRepo := repository.NewPubRepo(db)
	pubSvc := service.NewPubService(pubRepo, esClient)
	pubHdl := handler.NewPubHandler(pubSvc)
	pubHdl.RegisterRoutes(api)

	// 6) 注册 Gnc 模块
	gncRepo := repository.NewGncRepo(db)
	gncSvc := service.NewGncService(gncRepo)
	gncHdl := handler.NewGncHandler(gncSvc)
	gncHdl.RegisterRoutes(api)

	// 7) 初始化 Kafka & Snowflake
	//    从 pkg 包中获取初始化函数
	kafkaWriter := pkg.InitKafkaWriter(kafkaUrl, topicOrerCreate) // broker和topic可改
	snowflakeFn := pkg.InitSnowflake(1)
	// orderApi := proxy.NewOrderApi(map[string]string{
	// 	"CreateOrder": "https://api0.10000hk.com/api/product/gift/customer/orders/create",
	// 	"QueryOrder":  "https://api0.10000hk.com/api/product/gift/orders/query",
	// }, pubSvc)

	// 8) Order 模块
	orderRepo := repository.NewOrderRepo(db)
	// 这里的 orderSvc 是“只发Kafka” or “先插DB再发Kafka”，取决于order_service.go的模式
	orderSvc := service.NewOrderService(orderRepo, kafkaWriter, snowflakeFn /*, esClient*/)
	orderHdl := handler.NewOrderHandler(orderSvc, pubSvc)
	orderHdl.RegisterRoutes(api) // POST /orders, GET /orders/:orderId
	notifier := service.NewUpstreamNotifier("https://left.10000hk.com/api/order/upstream/update_order_status")
	// 2) Create the QueryScheduler
	scheduler := mq.NewQueryScheduler(100, notifier) // buffer size
	scheduler.Start()

	// 9) 若要在同进程启动消费端:
	//    初始化消费者, 并启动
	orderConsumer := mq.NewOrderConsumer(
		[]string{kafkaUrl}, // broker list
		topicOrerCreate,    // topic
		consumerId,         // group ID
		orderSvc,           // 注入同一个 orderSvc
		pubSvc,
		scheduler,
	)
	orderConsumer.Start()
	defer orderConsumer.Stop()

	// 10) 启动 Fiber
	addr := ":3001"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(app.Listen(addr))
}
