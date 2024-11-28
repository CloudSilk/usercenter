package model

import (
	"encoding/json"
	"errors"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type APP struct {
	model.Model
	TenantID    string    `json:"tenantID" gorm:"size:36;index" `
	ProjectID   string    `json:"projectID" gorm:"index;size:36"`
	Name        string    `json:"name" gorm:"size:200;index"`
	Entry       string    `json:"entry" gorm:"size:200;"`
	DevEntry    string    `json:"devEntry" gorm:"size:200;"`
	TestEntry   string    `json:"testEntry" gorm:"size:200;"`
	PreEntry    string    `json:"preEntry" gorm:"size:200;"`
	DisplayName string    `json:"displayName" gorm:"size:100;comment:显示名字"`
	Credentials bool      `json:"credentials"`
	Description string    `json:"description" gorm:"size:200;"`
	Props       []APPProp `json:"props"`
	Container   string    `json:"container" gorm:"size:100"`
	ActiveRule  string    `json:"activeRule" gorm:"size:200"`
	Enable      bool      `json:"enable"`
	IsMust      bool      `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

type APPProp struct {
	gorm.Model
	APPID uint   `json:"appID"`
	Key   string `json:"key" gorm:"size:50;"`
	Value string `json:"value" gorm:"size:200;"`
}

func CreateAPP(md *APP) error {
	duplication, err := dbClient.CreateWithCheckDuplication(md, "name = ?", md.Name)
	if err != nil {
		return err
	}
	if duplication {
		return errors.New("存在相同APP")
	}
	return nil
}

func DeleteAPP(id string) (err error) {
	return dbClient.DB().Unscoped().Delete(&APP{}, "id=?", id).Error
}

type QueryAPPRequest struct {
	model.CommonRequest
	Name       string `json:"name" form:"name" uri:"name"`
	Enable     int    `json:"enable" form:"enable" uri:"enable"`
	IsMust     bool   `json:"isMust" form:"isMust" uri:"isMust"`
	SortConfig string `json:"sortConfig" form:"sortConfig" uri:"sortConfig"`
}

type QueryAPPResponse struct {
	model.CommonResponse
	Data []*APP `json:"data"`
}

func QueryAPP(req *QueryAPPRequest, resp *QueryAPPResponse) {
	db := dbClient.DB().Model(&APP{})

	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}

	if req.Enable != -1 {
		db = db.Where("enable = ?", req.Enable == 1)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`name`")
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		return
	}

	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &resp.Data, nil)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	resp.Total = resp.Records
}

func GetAllAPPs() (mds []*APP, err error) {
	err = dbClient.DB().Preload("Props").Where("enable=1").Find(&mds).Error
	return
}

func GetAPPById(id string) (md APP, err error) {
	err = dbClient.DB().Preload("Props").Where("id = ?", id).First(&md).Error
	return
}

func UpdateAPP(md *APP) error {
	return dbClient.DB().Transaction(func(tx *gorm.DB) error {
		oldAPP := &APP{}
		err := tx.Preload("Props").Preload(clause.Associations).Where("id = ?", md.ID).First(oldAPP).Error
		if err != nil {
			return err
		}
		var deleteFile []uint
		for _, oldFile := range oldAPP.Props {
			flag := false
			for _, newFile := range md.Props {
				if newFile.ID == oldFile.ID {
					flag = true
				}
			}
			if !flag {
				deleteFile = append(deleteFile, oldFile.ID)
			}
		}
		if len(deleteFile) > 0 {
			err = tx.Unscoped().Delete(&APPProp{}, "id in ?", deleteFile).Error
			if err != nil {
				return err
			}
		}

		duplication, err := dbClient.UpdateWithCheckDuplicationAndOmit(tx, md, true, []string{"created_at"}, "id <> ? and name = ?", md.ID, md.Name)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("存在相同APP")
		}

		return nil
	})
}

func ExportAllAPPs(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&APP{}).Preload("Props").Preload(clause.Associations)

	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}

	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	var list []*APP
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}
