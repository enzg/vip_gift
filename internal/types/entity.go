// internal/types/entity.go
package types

import (
	"encoding/json"
	"time"

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

	PublicCode       string             `gorm:"column:public_code;size:50;not null;uniqueIndex" json:"publicCode"`
	Compositions     []PubComposeEntity `gorm:"-" json:"compositions"`
	SalePrice        float64            `gorm:"column:sale_price;not null;default:0"    json:"salePrice"`
	ParValue         float64            `gorm:"column:par_value;not null;default:0"     json:"parValue"`
	CommissionMF     float64            `gorm:"column:commission_mf;not null;default:0"  json:"commissionMF"`
	CommissionRuleMF string             `gorm:"column:commission_rule_mf;type:text"     json:"commissionRuleMF"`
	Cover            string             `gorm:"size:255"          json:"cover"`
	Desc             string             `gorm:"type:text"         json:"desc"`
	Pics             []string           `gorm:"-"                 json:"pics"`       // 不直接存库, 或另有处理
	PicsJSON         string             `gorm:"column:pics_json;type:text" json:"-"` // 内部持久化
	OriginData       string             `gorm:"type:text"         json:"originData"`
	Status           int64              `gorm:"not null;default:0" json:"status"` // 1上架,0下架,2其他
	ProductName      string             `gorm:"column:product_name;size:100;not null"   json:"productName"`
	Tag              string             `gorm:"column:tag;size:255"      json:"tag"`                 // 直接映射到 DB
	Categories       []string           `gorm:"-"                       json:"categories,omitempty"` // 不直接存表
	CategoriesJSON   string             `gorm:"column:categories_json;type:text"  json:"-"`          // 用于持久化 JSON
	Fetched          bool               `gorm:"-" json:"fetched,omitempty"`
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
	// 2) 序列化 categories => categories_json
	if d.Categories != nil {
		b, err := json.Marshal(d.Categories)
		if err != nil {
			return err
		}
		d.CategoriesJSON = string(b)
	} else {
		d.CategoriesJSON = "[]"
	}
	return nil
}

func (d *PubEntity) AfterFind(tx *gorm.DB) (err error) {
	// 1) 反序列化 pics
	if d.PicsJSON == "" {
		d.Pics = []string{}
	} else {
		var tmp []string
		if err := json.Unmarshal([]byte(d.PicsJSON), &tmp); err != nil {
			d.Pics = []string{}
		} else {
			d.Pics = tmp
		}
	}

	// 2) 反序列化 categories
	if d.CategoriesJSON == "" {
		d.Categories = []string{}
	} else {
		var cats []string
		if err := json.Unmarshal([]byte(d.CategoriesJSON), &cats); err != nil {
			d.Categories = []string{}
		} else {
			d.Categories = cats
		}
	}
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

// ------------------
// 4. OrderEntity (订单)
// ------------------

type OrderEntity struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderId           string    `gorm:"size:50;uniqueIndex"      json:"orderId"`           // 由 Snowflake 或其他方法生成
	UserSn            string    `gorm:"size:255"                  json:"userSn"`           // 用户编号
	ParentSn          string    `gorm:"size:255"                  json:"parentSn"`         // 上级编号
	DownstreamOrderId string    `gorm:"size:50;uniqueIndex"      json:"downstreamOrderId"` // 外部系统传入的订单ID
	DataJSON          string    `gorm:"type:text"                json:"dataJSON"`          // 存放订单相关数据
	Status            int64     `gorm:"not null;default:1"       json:"status"`            // 1=待处理, 2=完成, 3=取消等
	Remark            string    `gorm:"type:text"                json:"remark"`            // <-- 新增字段
	CommissionSelf    float64   `gorm:"not null;default:0"       json:"commissionSelf"`    // <-- 自购佣金
	CommissionParent  float64   `gorm:"not null;default:0"       json:"commissionParent"`  // <-- 上级佣金
	CommissionRule    string    `gorm:"size:255"                json:"commissionRule"`     // <-- MF YYF CYF 等
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (o OrderEntity) GetOrderId() string           { return o.OrderId }
func (o OrderEntity) GetDownstreamOrderId() string { return o.DownstreamOrderId }
func (o OrderEntity) GetDataJSON() string          { return o.DataJSON }
func (o OrderEntity) GetStatus() int64             { return o.Status }
