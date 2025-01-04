// internal/repository/gnc_repo.go
package repository

import (
	"10000hk.com/vip_gift/internal/types"
	"gorm.io/gorm"
)

type GncRepo interface {
	CreateGnc(ent *types.GncEntity) error
	GetGncByBaseCode(baseCode string) (*types.GncEntity, error)
	UpdateGnc(ent *types.GncEntity) error
	DeleteGncByBaseCode(baseCode string) error
	ListGnc(page, size int64) ([]types.GncEntity, int64, error) // 分页需求
}

type gncRepoImpl struct {
	db *gorm.DB
}

func NewGncRepo(db *gorm.DB) GncRepo {
	return &gncRepoImpl{db: db}
}

func (r *gncRepoImpl) CreateGnc(ent *types.GncEntity) error {
	return r.db.Create(ent).Error
}

func (r *gncRepoImpl) GetGncByBaseCode(baseCode string) (*types.GncEntity, error) {
	var entity types.GncEntity
	err := r.db.Where("base_code = ?", baseCode).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *gncRepoImpl) UpdateGnc(ent *types.GncEntity) error {
	return r.db.Save(ent).Error
}

func (r *gncRepoImpl) DeleteGncByBaseCode(baseCode string) error {
	return r.db.Where("base_code = ?", baseCode).Delete(&types.GncEntity{}).Error
}

// ListGnc 分页查询，page/size若=0，则返回全部
func (r *gncRepoImpl) ListGnc(page, size int64) ([]types.GncEntity, int64, error) {
	var list []types.GncEntity
	var total int64

	tx := r.db.Model(&types.GncEntity{})

	// 先统计总数
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page > 0 && size > 0 {
		offset := (page - 1) * size
		tx = tx.Offset(int(offset)).Limit(int(size))
	}
	// 查询数据
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}
