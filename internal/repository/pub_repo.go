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
	FindPubByNamePrefix(prefix string, pubs *[]types.PubEntity) error
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

	// 1) 统计总数 (分页需要)
	tx := r.db.Model(&types.PubEntity{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2) 应用分页
	if page > 0 && size > 0 {
		offset := (page - 1) * size
		tx = tx.Offset(int(offset)).Limit(int(size))
	}

	// 3) 查出所有 pub_entities
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, err
	}

	// 4) 如果需要“一并返回 Compositions”，我们手动再做一次查询:
	//    先收集所有 pub_entities 的 ID
	if len(list) == 0 {
		// 没有任何数据，直接返回空
		return list, total, nil
	}
	pubIDs := make([]uint64, 0, len(list))
	for _, pub := range list {
		pubIDs = append(pubIDs, pub.ID)
	}

	// 5) 一次性把所有关联的 PubComposeEntity 都捞出来
	//    where gift_public_id IN (我们所有的 pub ID)
	var comps []types.PubComposeEntity
	if err := r.db.Where("gift_public_id IN ?", pubIDs).
		Find(&comps).Error; err != nil {
		return nil, 0, err
	}

	// 6) 按 gift_public_id 分组存到 map 里
	compMap := make(map[uint64][]types.PubComposeEntity)
	for _, c := range comps {
		compMap[c.GiftPublicID] = append(compMap[c.GiftPublicID], c)
	}

	// 7) 回填到每个 PubEntity 的 Compositions 字段
	for i := range list {
		list[i].Compositions = compMap[list[i].ID]
	}

	return list, total, nil
}
func (r *pubRepoImpl) FindPubByNamePrefix(prefix string, pubs *[]types.PubEntity) error {
	// 例如: product_name LIKE '爱奇艺%'
	// prefix + "%"
	likeStr := prefix + "%"
	return r.db.Where("product_name LIKE ?", likeStr).
		Find(pubs).Error
}
