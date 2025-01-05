// internal/types/entity.go
package types

import (
	"encoding/json"

	"gorm.io/gorm"
)

// ------------------
// 1. GncEntity (基础权益品)
// ------------------
type GncEntity struct {
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	BaseCode     string   `gorm:"column:base_code;size:50;not null"       json:"publicCode"`
	ProductName  string   `gorm:"column:product_name;size:100;not null"   json:"productName"`
	ProductType  int64    `gorm:"column:product_type;not null;default:0"  json:"productType"`
	ParValue     float64  `gorm:"column:par_value;not null;default:0"     json:"parValue"`
	SalePrice    float64  `gorm:"column:sale_price;not null;default:0"    json:"salePrice"`
	IsShelve     int64    `gorm:"column:is_shelve;not null;default:0"     json:"isShelve"` // 0:下架, 1:上架
	ProductDesc  string   `gorm:"column:product_desc;type:text"           json:"productDesc"`
	ProductCover string   `gorm:"column:product_cover;size:255"           json:"productCover"`
	ProductPics  []string `gorm:"-"                                       json:"productPics"` // 示例: 不直接存表，或另存 JSON

	CallbackURL string `gorm:"column:callback_url;size:255" json:"callbackUrl"`
	QueryURL    string `gorm:"column:query_url;size:255"    json:"queryUrl"`

	OriginData string `gorm:"column:origin_data;type:text" json:"originData"`
}

// 实现 GiftBase 接口
func (f *GncEntity) GetBaseCode() string    { return f.BaseCode }
func (f *GncEntity) GetCallbackURL() string { return f.CallbackURL }
func (f *GncEntity) GetQueryURL() string    { return f.QueryURL }
func (f *GncEntity) GetStatus() int64       { return f.IsShelve }

func (f *GncEntity) BeforeCreate(tx *gorm.DB) (err error) {
	// you can do something before create
	return nil
}
func (f *GncEntity) BeforeUpdate(tx *gorm.DB) (err error) {
	// you can do something before update
	return nil
}

// ------------------
// 2. PubComposeEntity (组合项)
// ------------------
type PubComposeEntity struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	GiftPublicID uint64 `gorm:"index"                    json:"giftPublicId"`

	BaseCode string `gorm:"size:50;not null"   json:"baseCode"`
	Strategy string `gorm:"size:100;not null"  json:"strategy"`
	Snapshot string `gorm:"type:text;not null" json:"snapshot"`
}

// 实现 Composition 接口
func (d *PubComposeEntity) GetBaseCode() string { return d.BaseCode }
func (d *PubComposeEntity) GetSnapshot() string { return d.Snapshot }
func (d *PubComposeEntity) GetStrategy() string { return d.Strategy }

// ------------------
// 3. PubEntity (在售权益品)
// ------------------
type PubEntity struct {
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	PublicCode   string             `gorm:"column:public_code;size:50;not null;uniqueIndex" json:"publicCode"`
	Compositions []PubComposeEntity `gorm:"-" json:"compositions"`
	SalePrice    float64            `gorm:"column:sale_price;not null;default:0"    json:"salePrice"`
	ParValue     float64            `gorm:"column:par_value;not null;default:0"     json:"parValue"`
	Cover        string             `gorm:"size:255"          json:"cover"`
	Desc         string             `gorm:"type:text"         json:"desc"`
	Pics         []string           `gorm:"-"                 json:"pics"`       // 不直接存库, 或另有处理
	PicsJSON     string             `gorm:"column:pics_json;type:text" json:"-"` // 内部持久化
	OriginData   string             `gorm:"type:text"         json:"originData"`
	Status       int64              `gorm:"not null;default:0" json:"status"` // 1上架,0下架,2其他
	ProductName  string             `gorm:"column:product_name;size:100;not null"   json:"productName"`
	// Categories 不落库，仅在业务或搜索时使用
	Categories []string `gorm:"-" json:"categories,omitempty"`
	Tag        string   `gorm:"-" json:"tag,omitempty"`
}

// 实现 GiftPublic 接口
func (d *PubEntity) GetPublicCode() string {
	return d.PublicCode
}

func (d *PubEntity) GetCompositions() []Composition {
	result := make([]Composition, len(d.Compositions))
	for i := range d.Compositions {
		result[i] = &d.Compositions[i]
	}
	return result
}

func (d *PubEntity) BeforeSave(tx *gorm.DB) (err error) {
	if d.Pics != nil {
		b, err := json.Marshal(d.Pics)
		if err != nil {
			return err
		}
		d.PicsJSON = string(b)
	} else {
		d.PicsJSON = "[]"
	}
	return nil
}

func (d *PubEntity) AfterFind(tx *gorm.DB) (err error) {
	if d.PicsJSON == "" {
		d.Pics = []string{}
		return nil
	}
	var tmp []string
	if err := json.Unmarshal([]byte(d.PicsJSON), &tmp); err != nil {
		d.Pics = []string{}
		return nil
	}
	d.Pics = tmp
	return nil
}

// 实现 GiftSearchable
func (d *PubEntity) GetESID() string {
	// 以 PublicCode 作为 ES 文档 ID
	return d.PublicCode
}
func (d *PubEntity) GetESName() string {
	// 以 ProductName 作为搜索标题
	return d.ProductName
}
func (d *PubEntity) GetESCategories() []string {
	// 返回我们刚加的 Categories 字段
	return d.Categories
}
