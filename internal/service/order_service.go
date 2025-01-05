package service

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/segmentio/kafka-go" // 如需投递订单消息到 Kafka
    "gorm.io/gorm"

    "10000hk.com/vip_gift/internal/repository" // 引用 order_repo.go
    "10000hk.com/vip_gift/internal/types"       // 里有 OrderEntity, OrderDTO
    // "github.com/elastic/go-elasticsearch/v7"  // 如果需要写ES
)

// OrderService 定义订单业务接口
type OrderService interface {
    CreateOrder(ctx context.Context, dto *types.OrderDTO) (*types.OrderDTO, error)
    GetOrder(ctx context.Context, orderId string) (*types.OrderDTO, error)

    UpdateOrder(ctx context.Context, dto *types.OrderDTO) (*types.OrderDTO, error)
    DeleteOrder(ctx context.Context, orderId string) error
    ListOrder(ctx context.Context, page, size int64) ([]types.OrderDTO, int64, error)
}

// orderServiceImpl 实现 OrderService
type orderServiceImpl struct {
    repo        repository.OrderRepo
    kafkaWriter *kafka.Writer       // 可选, 用于写Kafka
    snowflakeFn func() string       // 可选, 用于生成 OrderId
    // esClient   *elasticsearch.Client // 如需写ES可在 NewOrderService 注入
}

// 确保实现关系
var _ OrderService = (*orderServiceImpl)(nil)

// NewOrderService 工厂函数
func NewOrderService(
    repo repository.OrderRepo,
    kWriter *kafka.Writer,
    sfFn func() string,
    // es *elasticsearch.Client
) OrderService {
    return &orderServiceImpl{
        repo:        repo,
        kafkaWriter: kWriter,
        snowflakeFn: sfFn,
        // esClient:  es,
    }
}

// -------------------------------------------------------------------
// 1) CreateOrder
// -------------------------------------------------------------------
func (s *orderServiceImpl) CreateOrder(ctx context.Context, dto *types.OrderDTO) (*types.OrderDTO, error) {
    // 基础校验
    if dto.DownstreamOrderId == "" {
        return nil, fmt.Errorf("downstreamOrderId is required")
    }
    if dto.DataJSON == "" {
        return nil, fmt.Errorf("dataJSON is required")
    }

    // 1) 生成订单ID (Snowflake)
    var orderId string
    if s.snowflakeFn != nil {
        orderId = s.snowflakeFn()
    } else {
        // 如果未注入snowflakeFn,也可用其他方式
        orderId = fmt.Sprintf("ORD-%d", generateRandom()) 
    }
    dto.OrderId = orderId

    // 2) 写数据库
    ent := &types.OrderEntity{
        OrderId:           orderId,
        DownstreamOrderId: dto.DownstreamOrderId,
        DataJSON:          dto.DataJSON,
        Status:            1, // 1=待处理
    }
    if err := s.repo.CreateOrder(ent); err != nil {
        return nil, fmt.Errorf("CreateOrder db error: %w", err)
    }

    // 3) 投递Kafka（可选）
    if s.kafkaWriter != nil {
        msgBytes, _ := json.Marshal(dto)
        err := s.kafkaWriter.WriteMessages(ctx, kafka.Message{
            Key:   []byte(orderId),
            Value: msgBytes,
        })
        if err != nil {
            return nil, fmt.Errorf("kafka produce error: %v", err)
        }
        log.Printf("order %s produced to Kafka\n", orderId)
    }

    // 4) 可选写ES
    // if s.esClient != nil {
    //     if err := s.indexToES(ent); err != nil {
    //         log.Printf("ES index error: %v\n", err)
    //     }
    // }

    // 回填状态
    dto.Status = ent.Status
    return dto, nil
}

// -------------------------------------------------------------------
// 2) GetOrder
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
// 3) UpdateOrder
// 假设下游只会修改 DataJSON / Status
// -------------------------------------------------------------------
func (s *orderServiceImpl) UpdateOrder(ctx context.Context, dto *types.OrderDTO) (*types.OrderDTO, error) {
    if dto.OrderId == "" {
        return nil, fmt.Errorf("orderId is required for update")
    }

    // 1) 查数据库
    ent, err := s.repo.GetOrderByOrderId(dto.OrderId)
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil, fmt.Errorf("订单不存在: %s", dto.OrderId)
        }
        return nil, err
    }

    // 2) 更新可修改字段
    if dto.DataJSON != "" {
        ent.DataJSON = dto.DataJSON
    }
    if dto.Status != 0 {
        ent.Status = dto.Status
    }

    // 3) 写数据库
    if err := s.repo.UpdateOrder(ent); err != nil {
        return nil, fmt.Errorf("UpdateOrder db error: %w", err)
    }

    // 4) 可选: 更新ES
    // if s.esClient != nil {
    //     if err := s.indexToES(ent); err != nil {
    //         log.Printf("ES update error: %v", err)
    //     }
    // }

    // 回填
    dto.DownstreamOrderId = ent.DownstreamOrderId
    dto.DataJSON = ent.DataJSON
    dto.Status = ent.Status

    return dto, nil
}

// -------------------------------------------------------------------
// 4) DeleteOrder
// -------------------------------------------------------------------
func (s *orderServiceImpl) DeleteOrder(ctx context.Context, orderId string) error {
    if orderId == "" {
        return fmt.Errorf("orderId is required")
    }
    // 调用 repo.DeleteOrderByOrderId
    err := s.repo.DeleteOrderByOrderId(orderId)
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return fmt.Errorf("订单不存在: %s", orderId)
        }
        return err
    }

    // 可选: 同步ES -> s.deleteFromES(orderId)
    return nil
}

// -------------------------------------------------------------------
// 5) ListOrder
// -------------------------------------------------------------------
func (s *orderServiceImpl) ListOrder(ctx context.Context, page, size int64) ([]types.OrderDTO, int64, error) {
    ents, total, err := s.repo.ListOrder(page, size)
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

// -------------------------------------------------------------------
// 如果要写ES可加 indexToES / deleteFromES
// -------------------------------------------------------------------
// func (s *orderServiceImpl) indexToES(ent *types.OrderEntity) error {
//     doc := map[string]interface{}{
//         "id":         ent.ID,
//         "orderId":    ent.OrderId,
//         "downstream": ent.DownstreamOrderId,
//         "dataJSON":   ent.DataJSON,
//         "status":     ent.Status,
//         "created_at": time.Now().Format(time.RFC3339),
//         "updated_at": time.Now().Format(time.RFC3339),
//     }
//     bodyBytes, _ := json.Marshal(doc)
//     req := esapi.IndexRequest{
//         Index:      "orders_index",
//         DocumentID: ent.OrderId,
//         Body:       bytes.NewReader(bodyBytes),
//         Refresh:    "true",
//     }
//     resp, err := req.Do(context.Background(), s.esClient)
//     if err != nil {
//         return err
//     }
//     defer resp.Body.Close()
//     if resp.IsError() {
//         return fmt.Errorf("ES index error: %s", resp.Status())
//     }
//     return nil
// }

// func (s *orderServiceImpl) deleteFromES(orderId string) error {
//     req := esapi.DeleteRequest{
//         Index:      "orders_index",
//         DocumentID: orderId,
//         Refresh:    "true",
//     }
//     resp, err := req.Do(context.Background(), s.esClient)
//     if err != nil {
//         return err
//     }
//     defer resp.Body.Close()
//     if resp.IsError() && resp.StatusCode != 404 {
//         return fmt.Errorf("ES delete error: %s", resp.Status())
//     }
//     return nil
// }

// 额外可写: generateRandom() 仅示例
func generateRandom() int64 {
    // pseudo random code, or use crypto/rand
    return 100000 + int64(len("order"))
}