// internal/repository/pub_repo.go
package repository

import (
	"10000hk.com/vip_gift/internal/types"
	"gorm.io/gorm"
)

type PubRepo interface {
	CreatePub(ent *types.PubEntity) error
	GetPubByPublicCode(publicCode string) (*types.PubEntity, error)
	UpdatePub(ent *types.PubEntity) error
	DeletePubByPublicCode(publicCode string) error

	ListPub(page, size int64) ([]types.PubEntity, int64, error) // 分页需求
}

type pubRepoImpl struct {
	db *gorm.DB
}

func NewPubRepo(db *gorm.DB) PubRepo {
	return &pubRepoImpl{db: db}
}

func (r *pubRepoImpl) CreatePub(ent *types.PubEntity) error {
	// 1) 先创建主记录
	if err := r.db.Create(ent).Error; err != nil {
		return err
	}

	// 2) 再处理 Compositions
	//    如果 ent.Compositions 不为空，我们手动给 each GiftPublicID = ent.ID
	//    并单独执行 Create
	if len(ent.Compositions) > 0 {
		for i := range ent.Compositions {
			ent.Compositions[i].GiftPublicID = ent.ID
		}
		if err := r.db.Create(&ent.Compositions).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *pubRepoImpl) GetPubByPublicCode(publicCode string) (*types.PubEntity, error) {
	var entity types.PubEntity
	// 1) 先查 pub_entities
	err := r.db.Where("public_code = ?", publicCode).First(&entity).Error
	if err != nil {
		return nil, err
	}

	// 2) 再查 pub_compose_entities => gift_public_id = entity.ID
	var comps []types.PubComposeEntity
	if err := r.db.Where("gift_public_id = ?", entity.ID).Find(&comps).Error; err != nil {
		return nil, err
	}
	entity.Compositions = comps
	return &entity, nil
}

func (r *pubRepoImpl) UpdatePub(ent *types.PubEntity) error {
	// 1) 更新主记录
	if err := r.db.Save(ent).Error; err != nil {
		return err
	}

	// 2) 更新 Compositions:
	//    业务需要：是全部删除后重插，还是增删改？
	//    这里给示例：先全部delete gift_public_id=ent.ID，再批量插入 ent.Compositions
	if err := r.db.Where("gift_public_id = ?", ent.ID).Delete(&types.PubComposeEntity{}).Error; err != nil {
		return err
	}
	if len(ent.Compositions) > 0 {
		for i := range ent.Compositions {
			ent.Compositions[i].GiftPublicID = ent.ID
		}
		if err := r.db.Create(&ent.Compositions).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *pubRepoImpl) DeletePubByPublicCode(publicCode string) error {
	// return r.db.Where("public_code = ?", publicCode).Delete(&types.PubEntity{}).Error
	// 1) 找到主记录
	var entity types.PubEntity
	err := r.db.Where("public_code = ?", publicCode).First(&entity).Error
	if err != nil {
		return err
	}

	// 2) 删除对应 Compositions
	if err := r.db.Where("gift_public_id = ?", entity.ID).Delete(&types.PubComposeEntity{}).Error; err != nil {
		return err
	}

	// 3) 删除主记录
	if err := r.db.Delete(&entity).Error; err != nil {
		return err
	}
	return nil
}

func (r *pubRepoImpl) ListPub(page, size int64) ([]types.PubEntity, int64, error) {
	var list []types.PubEntity
	var total int64

	tx := r.db.Model(&types.PubEntity{})

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page > 0 && size > 0 {
		offset := (page - 1) * size
		tx = tx.Offset(int(offset)).Limit(int(size))
	}

	// 查询 pub_entities
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	// 如果你需要把 Compositions 一并返回，则需要手动查找
	// 这可能需要一个map: pub_id -> []compositions
	// 这里示例就不写了，看你的业务需求
	// ...
	return list, total, nil
}
