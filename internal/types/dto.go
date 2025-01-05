// internal/types/dto.go
package types

import (
	"github.com/jinzhu/copier"
)

// ------------------
// 1. GncDTO
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

// FromEntity uses helper to unify the logic
func (dto *GncDTO) FromEntity(ent *GncEntity) error {
	if ent == nil {
		return nil
	}
	newDTO, err := copyGncEntityToDTO(ent)
	if err != nil {
		return err
	}
	*dto = *newDTO
	return nil
}

func (dto *GncDTO) ToEntity() (*GncEntity, error) {
	return copyGncDTOToEntity(dto)
}

// ------------------
// 2. PubComposeDTO
// ------------------
type PubComposeDTO struct {
	ID           uint64 `json:"id"`
	GiftPublicID uint64 `json:"giftPublicId"`
	BaseCode     string `json:"baseCode"`
	Strategy     string `json:"strategy"`
	Snapshot     string `json:"snapshot"`
}

// FromEntity uses helper
func (dto *PubComposeDTO) FromEntity(ent *PubComposeEntity) error {
	if ent == nil {
		return nil
	}
	newDTO, err := copyPubComposeEntityToDTO(ent)
	if err != nil {
		return err
	}
	*dto = *newDTO
	return nil
}

func (dto *PubComposeDTO) ToEntity() (*PubComposeEntity, error) {
	return copyPubComposeDTOToEntity(dto)
}

// ------------------
// 3. PubDTO
// ------------------
type PubDTO struct {
	PublicCode  string   `json:"publicCode"`
	ProductName string   `json:"productName"`
	SalePrice   float64  `json:"salePrice"`
	ParValue    float64  `json:"parValue"`
	Cover       string   `json:"cover"`
	Desc        string   `json:"desc"`
	Pics        []string `json:"pics"`

	Categories []string `json:"categories,omitempty"` // Tags / categories for ES
	Tag        string   `json:"tag,omitempty"`
	Status     int64    `json:"status"`
	OriginData string   `json:"originData"`

	Compositions []PubComposeDTO `json:"compositions,omitempty"`
	Fetched      bool            `json:"fetched"`
}

func (dto *PubDTO) FromEntity(ent *PubEntity) error {
	if ent == nil {
		return nil
	}
	newDTO, err := copyPubEntityToDTO(ent)
	if err != nil {
		return err
	}
	*dto = *newDTO
	return nil
}

func (dto *PubDTO) ToEntity() (*PubEntity, error) {
	return copyPubDTOToEntity(dto)
}

// =====================================================================
// HELPER FUNCTIONS (Private) - One place to unify the logic
// =====================================================================

// ---------- GNC HELPER ----------
func copyGncEntityToDTO(ent *GncEntity) (*GncDTO, error) {
	dto := &GncDTO{}
	if err := copier.Copy(dto, ent); err != nil {
		return nil, err
	}
	// If you have special logic for productPics, etc, do it here
	// e.g. if ent.ProductPicsJSON -> unmarshal -> dto.ProductPics
	return dto, nil
}

func copyGncDTOToEntity(dto *GncDTO) (*GncEntity, error) {
	ent := &GncEntity{}
	if err := copier.Copy(ent, dto); err != nil {
		return nil, err
	}
	// If you have special logic for pics or others, do it here
	return ent, nil
}

// ---------- PubCompose HELPER ----------
func copyPubComposeEntityToDTO(ent *PubComposeEntity) (*PubComposeDTO, error) {
	dto := &PubComposeDTO{}
	if err := copier.Copy(dto, ent); err != nil {
		return nil, err
	}
	return dto, nil
}

func copyPubComposeDTOToEntity(dto *PubComposeDTO) (*PubComposeEntity, error) {
	ent := &PubComposeEntity{}
	if err := copier.Copy(ent, dto); err != nil {
		return nil, err
	}
	return ent, nil
}

// ---------- Pub HELPER ----------
func copyPubEntityToDTO(ent *PubEntity) (*PubDTO, error) {
	dto := &PubDTO{}
	if err := copier.Copy(dto, ent); err != nil {
		return nil, err
	}
	// Now do manual copying for Compositions (nested slice)
	if len(ent.Compositions) > 0 {
		comps := make([]PubComposeDTO, len(ent.Compositions))
		for i, ce := range ent.Compositions {
			// re-use helper
			cDto, err := copyPubComposeEntityToDTO(&ce)
			if err != nil {
				return nil, err
			}
			comps[i] = *cDto
		}
		dto.Compositions = comps
	}
	return dto, nil
}

func copyPubDTOToEntity(dto *PubDTO) (*PubEntity, error) {
	ent := &PubEntity{}
	if err := copier.Copy(ent, dto); err != nil {
		return nil, err
	}
	// handle Compositions manually
	if len(dto.Compositions) > 0 {
		comps := make([]PubComposeEntity, len(dto.Compositions))
		for i, c := range dto.Compositions {
			cEnt, err := copyPubComposeDTOToEntity(&c)
			if err != nil {
				return nil, err
			}
			comps[i] = *cEnt
		}
		ent.Compositions = comps
	}
	return ent, nil
}
