package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"10000hk.com/vip_gift/internal/types"
)

// OrderRepo 定义订单相关的数据库操作接口
type OrderRepo interface {
	// CreateOrder 插入一条订单记录
	CreateOrder(ent *types.OrderEntity) error

	// GetOrderByOrderId 根据内部OrderId查询
	GetOrderByOrderId(orderId string) (*types.OrderEntity, error)

	// UpdateOrder 更新订单
	UpdateOrder(ent *types.OrderEntity) error

	// DeleteOrderByOrderId 根据OrderId删除订单
	DeleteOrderByOrderId(orderId string) error

	// ListOrder 分页列出订单
	ListOrder(page, size int64) ([]types.OrderEntity, int64, error)
}

// orderRepoImpl 实现 OrderRepo 接口
type orderRepoImpl struct {
	db *gorm.DB
}

// NewOrderRepo 初始化
func NewOrderRepo(db *gorm.DB) OrderRepo {
	return &orderRepoImpl{db: db}
}

// -------------------- 实现接口方法 --------------------

// CreateOrder 插入一条记录
func (r *orderRepoImpl) CreateOrder(ent *types.OrderEntity) error {
	if err := r.db.Create(ent).Error; err != nil {
		return errors.Join(err, errors.New("CreateOrder db error"))
	}
	return nil
}

// GetOrderByOrderId 根据 OrderId 获取订单
func (r *orderRepoImpl) GetOrderByOrderId(orderId string) (*types.OrderEntity, error) {
	var order types.OrderEntity
	if err := r.db.Where("order_id = ?", orderId).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("订单不存在, orderId=%s", orderId)
		}
		return nil, errors.Join(err, errors.New("GetOrderByOrderId db error"))
	}
	return &order, nil
}

// UpdateOrder 更新订单
func (r *orderRepoImpl) UpdateOrder(ent *types.OrderEntity) error {
	if err := r.db.Save(ent).Error; err != nil {
		return errors.Join(err, errors.New("UpdateOrder db error"))
	}
	return nil
}

// DeleteOrderByOrderId 根据 orderId 删除订单
func (r *orderRepoImpl) DeleteOrderByOrderId(orderId string) error {
	// 先查记录
	var order types.OrderEntity
	if err := r.db.Where("order_id = ?", orderId).First(&order).Error; err != nil {
		return errors.Join(err, errors.New("DeleteOrderByOrderId find db error"))
	}
	// 再删
	if err := r.db.Delete(&order).Error; err != nil {
		return errors.Join(err, errors.New("DeleteOrderByOrderId db error"))
	}
	return nil
}

// ListOrder 简易分页列出订单
func (r *orderRepoImpl) ListOrder(page, size int64) ([]types.OrderEntity, int64, error) {
	var list []types.OrderEntity
	var total int64

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	tx := r.db.Model(&types.OrderEntity{})

	// 统计总数
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errors.Join(err, errors.New("ListOrder count error"))
	}
	offset := (page - 1) * size
	// 分页查询
	if err := tx.Offset(int(offset)).Limit(int(size)).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, 0, errors.Join(err, errors.New("ListOrder find error"))
	}

	return list, total, nil
}