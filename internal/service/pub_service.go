// internal/service/pub_service.go
package service

import (
	"errors"

	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/types"
)

type PubService interface {
	Create(dto *types.PubDTO) (*types.PubDTO, error)
	GetByPublicCode(publicCode string) (*types.PubDTO, error)
	UpdateByPublicCode(publicCode string, dto *types.PubDTO) (*types.PubDTO, error)
	DeleteByPublicCode(publicCode string) error

	List(page, size int64) ([]types.PubDTO, int64, error)
}

type pubServiceImpl struct {
	repo repository.PubRepo
}

func NewPubService(repo repository.PubRepo) PubService {
	return &pubServiceImpl{repo: repo}
}

func (s *pubServiceImpl) Create(dto *types.PubDTO) (*types.PubDTO, error) {
	if dto.PublicCode == "" {
		return nil, errors.New("publicCode is required")
	}
	ent, err := dto.ToEntity()
	if err != nil {
		return nil, err
	}
	// 默认上架
	if ent.Status == 0 {
		ent.Status = 1
	}
	if err := s.repo.CreatePub(ent); err != nil {
		return nil, err
	}
	_ = dto.FromEntity(ent)
	return dto, nil
}

func (s *pubServiceImpl) GetByPublicCode(publicCode string) (*types.PubDTO, error) {
	ent, err := s.repo.GetPubByPublicCode(publicCode)
	if err != nil {
		return nil, err
	}
	var dto types.PubDTO
	_ = dto.FromEntity(ent)
	return &dto, nil
}

// func (s *pubServiceImpl) UpdateByPublicCode(publicCode string, dto *types.PubDTO) (*types.PubDTO, error) {
// 	oldEnt, err := s.repo.GetPubByPublicCode(publicCode)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if dto.SalePrice != 0 {
// 		oldEnt.SalePrice = dto.SalePrice
// 	}
// 	if dto.ParValue != 0 {
// 		oldEnt.ParValue = dto.ParValue
// 	}
// 	if dto.Cover != "" {
// 		oldEnt.Cover = dto.Cover
// 	}
// 	if dto.Desc != "" {
// 		oldEnt.Desc = dto.Desc
// 	}
// 	if dto.OriginData != "" {
// 		oldEnt.OriginData = dto.OriginData
// 	}
// 	if dto.Status != 0 {
// 		oldEnt.Status = dto.Status
// 	}
// 	// 如果要更新 Compositions, 也可以做更复杂的处理

//		if err := s.repo.UpdatePub(oldEnt); err != nil {
//			return nil, err
//		}
//		var updated types.PubDTO
//		_ = updated.FromEntity(oldEnt)
//		return &updated, nil
//	}
func (s *pubServiceImpl) UpdateByPublicCode(publicCode string, dto *types.PubDTO) (*types.PubDTO, error) {
	// 1) 先查旧记录
	oldEnt, err := s.repo.GetPubByPublicCode(publicCode)
	if err != nil {
		return nil, err
	}

	// 2) 根据 dto 中的字段来更新 oldEnt
	if dto.SalePrice != 0 {
		oldEnt.SalePrice = dto.SalePrice
	}
	if dto.ParValue != 0 {
		// 如果你在 pub_entity 中定义了 ParValue 字段
		oldEnt.ParValue = dto.ParValue
	}
	if dto.Cover != "" {
		oldEnt.Cover = dto.Cover
	}
	if dto.Desc != "" {
		oldEnt.Desc = dto.Desc
	}
	if dto.OriginData != "" {
		oldEnt.OriginData = dto.OriginData
	}
	if dto.Status != 0 {
		oldEnt.Status = dto.Status
	}

	// 3) 更新 Compositions:
	//    如果本次 API 要**完全替换**原有的组合项，可以直接把 dto.Compositions 赋给 oldEnt.Compositions
	//    (后续 Repository 层将做“先删后增”或其他逻辑)

	if len(dto.Compositions) > 0 {
		// 这里假设你在 dto.ToEntity() 里也有类似逻辑，
		// 可以把 dto.Compositions 转成 []PubComposeEntity
		// 或者你可以手动拷贝

		// 示例：直接手动拷贝
		newComps := make([]types.PubComposeEntity, len(dto.Compositions))
		for i, cDto := range dto.Compositions {
			newComps[i].BaseCode = cDto.BaseCode
			newComps[i].Strategy = cDto.Strategy
			newComps[i].Snapshot = cDto.Snapshot
			// GiftPublicID 暂时不赋值，在 Repo 更新时再补 ent.ID
		}
		oldEnt.Compositions = newComps
	} else {
		// 若前端没带 compositions，可能意味着不更新它们
		// 或者意味着要清空组合项？看你的业务需求决定
		// 这里先示例：如果前端传空，就清空
		oldEnt.Compositions = nil
	}

	// 4) 调用 Repository 更新数据库
	if err := s.repo.UpdatePub(oldEnt); err != nil {
		return nil, err
	}

	// 5) 返回更新后的 DTO
	var updated types.PubDTO
	_ = updated.FromEntity(oldEnt)
	return &updated, nil
}

func (s *pubServiceImpl) DeleteByPublicCode(publicCode string) error {
	return s.repo.DeletePubByPublicCode(publicCode)
}

func (s *pubServiceImpl) List(page, size int64) ([]types.PubDTO, int64, error) {
	ents, total, err := s.repo.ListPub(page, size)
	if err != nil {
		return nil, 0, err
	}
	result := make([]types.PubDTO, len(ents))
	for i, e := range ents {
		_ = result[i].FromEntity(&e)
	}
	return result, total, nil
}
