// internal/types/dto.go
package types

import (
	"strings"

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
	ID               uint64   `json:"id"`
	PublicCode       string   `json:"publicCode"`
	ProductName      string   `json:"productName"`
	SalePrice        float64  `json:"salePrice"`
	ParValue         float64  `json:"parValue"`
	CommissionMF     float64  `json:"commissionMF"`
	CommissionRuleMF string   `json:"commissionRuleMF"`
	Cover            string   `json:"cover"`
	Desc             string   `json:"desc"`
	Pics             []string `json:"pics"`

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

// ------------------
// 4. OrderDTO
// ------------------
type OrderDTO struct {
	DownstreamOrderId string      `json:"downstreamOrderId"`
	PublicCode        string      `json:"publicCode"`
	DataJSON          string      `json:"dataJSON"`
	OrderId           string      `json:"orderId"`
	Status            OrderStatus `json:"status"`
	Remark            string      `json:"remark"`                     // 新增
	CommissionRule    string      `json:"commissionRule,omitempty"`   // MF CYF YYF
	UserSn            string      `json:"userSn,omitempty"`           // 用户编号
	ParentSn          string      `json:"parentSn,omitempty"`         // 上级编号
	CommissionSelf    float64     `json:"commissionSelf,omitempty"`   // 自己的佣金
	CommissionParent  float64     `json:"commissionParent,omitempty"` // 上级的佣金
	TradeStatus       string      `json:"tradeStatus,omitempty"`
	RefundStatus      string      `json:"refundStatus,omitempty"`
	DeliveryStatus    int64       `json:"deliveryStatus,omitempty"`
	SettlementStatus  int64       `json:"settlementStatus,omitempty"`
	Channel           string      `json:"channel,omitempty"` // 渠道
}
type ClientOrderDTO struct {
	*OrderDTO
	StatusText string `json:"statusText"`
}

func (dto *OrderDTO) FromEntity(ent *OrderEntity) error {
	if ent == nil {
		return nil
	}
	newDTO, err := copyOrderEntityToDTO(ent)
	if err != nil {
		return err
	}
	*dto = *newDTO
	return nil
}

func (dto *OrderDTO) ToEntity() (*OrderEntity, error) {
	return copyOrderDTOToEntity(dto)
}

func (dto *OrderDTO) ToClientDTO() (*ClientOrderDTO, error) {
	clientDTO := &ClientOrderDTO{
		OrderDTO:   dto,
		StatusText: dto.Status.String(),
	}
	return clientDTO, nil
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

// ---------- Order HELPER ----------
func copyOrderEntityToDTO(ent *OrderEntity) (*OrderDTO, error) {
	dto := &OrderDTO{}
	if err := copier.Copy(dto, ent); err != nil {
		return nil, err
	}
	return dto, nil
}

func copyOrderDTOToEntity(dto *OrderDTO) (*OrderEntity, error) {
	ent := &OrderEntity{}
	if err := copier.Copy(ent, dto); err != nil {
		return nil, err
	}
	return ent, nil
}

func GetChannel(s string) string {
	// 如果以 R10 或 R30 结尾，返回 "66hou.cn"
	if strings.HasSuffix(s, "R10") || strings.HasSuffix(s, "R30") {
		return "66hou.cn"
	}
	// 其他情况返回 "ytjb.cc"
	return "ytjb.cc"
}
