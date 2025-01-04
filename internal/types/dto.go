// internal/types/dto.go
package types

import (
	"github.com/jinzhu/copier"
)

// ------------------
// 1. GncDTO (基础权益品的对外数据)
// ------------------
type GncDTO struct {
	ID           uint64   `json:"id"`
	BaseCode     string   `json:"baseCode"`
	ProductName  string   `json:"productName"`
	ProductType  int64    `json:"productType"`
	ParValue     float64  `json:"parValue"`
	SalePrice    float64  `json:"salePrice"`
	IsShelve     int64    `json:"isShelve"`
	ProductDesc  string   `json:"productDesc"`
	ProductCover string   `json:"productCover"`
	ProductPics  []string `json:"productPics"`

	CallbackURL string `json:"callbackUrl"`
	QueryURL    string `json:"queryUrl"`

	OriginData string `json:"originData"`
}

func (dto *GncDTO) FromEntity(ent *GncEntity) error {
	return copier.Copy(dto, ent)
}
func (dto *GncDTO) ToEntity() (*GncEntity, error) {
	var ent GncEntity
	err := copier.Copy(&ent, dto)
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

// ------------------
// 2. PubComposeDTO (组合项的对外数据)
// ------------------
type PubComposeDTO struct {
	ID           uint64 `json:"id"`
	GiftPublicID uint64 `json:"giftPublicId"`
	BaseCode     string `json:"baseCode"`
	Strategy     string `json:"strategy"`
	Snapshot     string `json:"snapshot"`
}

func (dto *PubComposeDTO) FromEntity(ent *PubComposeEntity) error {
	return copier.Copy(dto, ent)
}
func (dto *PubComposeDTO) ToEntity() (*PubComposeEntity, error) {
	var ent PubComposeEntity
	err := copier.Copy(&ent, dto)
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

// ------------------
// 3. PubDTO (在售权益品的对外数据)
// ------------------
type PubDTO struct {
	ID           uint64          `json:"id"`
	PublicCode   string          `json:"publicCode"`
	Compositions []PubComposeDTO `json:"compositions"`
	ParValue     float64         `json:"parValue"`
	SalePrice    float64         `json:"salePrice"`
	Cover        string          `json:"cover"`
	Desc         string          `json:"desc"`
	Pics         []string        `json:"pics"`
	OriginData   string          `json:"originData"`
	Status       int64           `json:"status"`
	ProductName  string          `json:"productName"`
}

// FromEntity -> Dto
func (dto *PubDTO) FromEntity(ent *PubEntity) error {
	if err := copier.Copy(dto, ent); err != nil {
		return err
	}

	// 组合项
	dto.Compositions = make([]PubComposeDTO, len(ent.Compositions))
	for i, cEnt := range ent.Compositions {
		_ = copier.Copy(&dto.Compositions[i], &cEnt)
	}

	return nil
}

// ToEntity -> Entity
func (dto *PubDTO) ToEntity() (*PubEntity, error) {
	var ent PubEntity
	if err := copier.Copy(&ent, dto); err != nil {
		return nil, err
	}

	ent.Compositions = make([]PubComposeEntity, len(dto.Compositions))
	for i, cDto := range dto.Compositions {
		_ = copier.Copy(&ent.Compositions[i], &cDto)
	}

	return &ent, nil
}
