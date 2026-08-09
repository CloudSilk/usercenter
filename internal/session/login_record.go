package session

import (
	"context"
	"strings"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
)

const (
	LoginResultSuccess   = "SUCCESS"
	LoginResultFailed    = "FAILED"
	LoginResultChallenge = "MFA_REQUIRED"
)

type LoginRecord struct {
	commonmodel.Model
	PrincipalID   string `json:"principalID" gorm:"index;size:36"`
	TenantID      string `json:"tenantID" gorm:"index;size:36"`
	UserName      string `json:"userName" gorm:"index;size:100"`
	SessionID     string `json:"sessionID" gorm:"index;size:36"`
	AuthMethod    string `json:"authMethod" gorm:"index;size:30"`
	MFAUsed       bool   `json:"mfaUsed" gorm:"index;default:false"`
	DeviceType    int32  `json:"deviceType" gorm:"index"`
	DeviceName    string `json:"deviceName" gorm:"size:200"`
	IP            string `json:"ip" gorm:"index;size:50"`
	UserAgent     string `json:"userAgent" gorm:"size:500"`
	Location      string `json:"location" gorm:"size:200"`
	Result        string `json:"result" gorm:"index;size:30"`
	ResultCode    int32  `json:"resultCode" gorm:"index"`
	Message       string `json:"message" gorm:"size:500"`
	Abnormal      bool   `json:"abnormal" gorm:"index;default:false"`
	AnomalyReason string `json:"anomalyReason" gorm:"size:200"`
	PreviousIP    string `json:"previousIP" gorm:"size:50"`
	RequestID     string `json:"requestID" gorm:"index;size:100"`
}

func (LoginRecord) TableName() string { return "login_record" }

type LoginRecordQuery struct {
	PrincipalID string
	TenantID    string
	Keyword     string
	Result      string
	Abnormal    *bool
	PageIndex   int64
	PageSize    int64
}

type LoginRecordView struct {
	LoginRecord
	TenantName string `json:"tenantName"`
}

func RecordLogin(record *LoginRecord) {
	if record == nil || store.DB() == nil {
		return
	}
	if err := store.DB().Create(record).Error; err != nil {
		log.Errorf(context.Background(), "record login failed: %v", err)
	}
}

func QueryLoginRecords(query LoginRecordQuery) (list []*LoginRecordView, total int64, err error) {
	database := store.DB()
	if database == nil {
		return nil, 0, errStoreUnavailable
	}
	db := database.Model(&LoginRecord{})
	if query.PrincipalID != "" {
		db = db.Where("principal_id = ?", query.PrincipalID)
	}
	if query.TenantID != "" {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.Result != "" {
		db = db.Where("result = ?", strings.ToUpper(query.Result))
	}
	if query.Abnormal != nil {
		db = db.Where("abnormal = ?", *query.Abnormal)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tenantTable := database.NamingStrategy.TableName("Tenant")
		tenantIDs := database.Table(tenantTable).Select("id").Where("name LIKE ?", like)
		db = db.Where(
			"user_name LIKE ? OR ip LIKE ? OR device_name LIKE ? OR tenant_id IN (?)",
			like, like, like, tenantIDs,
		)
	}
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	pageIndex, pageSize := normalizePage(query.PageIndex, query.PageSize)
	var records []*LoginRecord
	if err = db.Order("created_at desc").
		Offset(int((pageIndex - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	tenantNames := make(map[string]string)
	if len(records) > 0 {
		tenantIDs := make([]string, 0, len(records))
		for _, item := range records {
			tenantIDs = append(tenantIDs, item.TenantID)
		}
		var tenantRows []struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err = database.Table(database.NamingStrategy.TableName("Tenant")).
			Select("id, name").
			Where("id IN ?", tenantIDs).
			Scan(&tenantRows).Error; err != nil {
			return nil, 0, err
		}
		for _, row := range tenantRows {
			tenantNames[row.ID] = row.Name
		}
	}
	list = make([]*LoginRecordView, 0, len(records))
	for _, item := range records {
		list = append(list, &LoginRecordView{LoginRecord: *item, TenantName: tenantNames[item.TenantID]})
	}
	return list, total, nil
}
