// Package prompt 提供 Prompt 模板管理：按租户存储可复用 Prompt，支持 {{变量}} 渲染。
//
// 配合 AI 网关使用：应用拉取模板 → 渲染变量 → 调 /v1/chat/completions，
// 把"Prompt 工程"与"模型调用"分离，模板版本化集中管理。
package prompt

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/store"
)

// PromptVersion 模板历史版本快照。
// Create 写入第 1 版，Update 写入新版本行，保留旧版本可供回滚查阅。
type PromptVersion struct {
	commonmodel.Model
	PromptID    string `json:"promptID" gorm:"index;size:36;comment:关联模板ID"`
	Version     int    `json:"version" gorm:"comment:版本号"`
	Name        string `json:"name" gorm:"size:100"`
	Category    string `json:"category" gorm:"size:50"`
	Description string `json:"description" gorm:"size:500"`
	Content     string `json:"content" gorm:"type:text"`
	Variables   string `json:"variables" gorm:"size:500"`
	ModelAlias  string `json:"modelAlias" gorm:"size:100"`
	ChangeNote  string `json:"changeNote" gorm:"size:500;comment:变更说明"`
}

func (PromptVersion) TableName() string { return "prompt_template_version" }
type PromptTemplate struct {
	commonmodel.Model
	TenantID    string `json:"tenantID" gorm:"index;size:36"`
	Name        string `json:"name" gorm:"size:100;index;comment:模板名称"`
	Category    string `json:"category" gorm:"size:50;index;comment:分类(system/agent/rag等)"`
	Description string `json:"description" gorm:"size:500"`
	Content     string `json:"content" gorm:"type:text;comment:模板正文，含 {{变量}} 占位"`
	Variables   string `json:"variables" gorm:"size:500;comment:逗号分隔的变量名清单"`
	ModelAlias  string `json:"modelAlias" gorm:"size:100;comment:建议模型别名"`
	Enable      bool   `json:"enable" gorm:"index;default:true"`
	Version     int    `json:"version" gorm:"default:1;comment:当前版本号"`
}

func (PromptTemplate) TableName() string { return "prompt_template" }

// Create 创建模板（同时写入第 1 版本快照）。
func Create(p *PromptTemplate) (string, error) {
	p.Variables = normalizeVariables(p.Content, p.Variables)
	if p.Version < 1 {
		p.Version = 1
	}
	if err := store.DB().Create(p).Error; err != nil {
		return "", err
	}
	_ = snapshotVersion(p, "", p.Version)
	return p.ID, nil
}

// Update 更新模板（写入新版本号 + 历史快照，不覆盖历史）。
func Update(p *PromptTemplate) error {
	return UpdateWithNote(p, "")
}

// UpdateWithNote 更新模板并记录变更说明。
// 流程：读取旧版本号 → 把新内容写主表并把 version+1 → 旧内容入历史快照。
func UpdateWithNote(p *PromptTemplate, changeNote string) error {
	p.Variables = normalizeVariables(p.Content, p.Variables)

	var current PromptTemplate
	if err := store.DB().Select("id, version").First(&current, "id = ?", p.ID).Error; err != nil {
		return err
	}
	// 先把当前线上版本存入历史（变更前快照）
	if err := snapshotVersion(&current, changeNote, current.Version); err != nil {
		return err
	}
	p.Version = current.Version + 1
	return store.DB().Model(&PromptTemplate{}).Where("id = ?", p.ID).
		Updates(map[string]interface{}{
			"name": p.Name, "category": p.Category, "description": p.Description,
			"content": p.Content, "variables": p.Variables, "model_alias": p.ModelAlias,
			"enable": p.Enable, "version": p.Version,
		}).Error
}

// snapshotVersion 把一个模板状态写入历史版本表。
func snapshotVersion(p *PromptTemplate, changeNote string, version int) error {
	if version < 1 {
		version = 1
	}
	// 仅快照字段齐全时落盘（Create 时 content 等已在 p 上）
	var full PromptTemplate
	if err := store.DB().First(&full, "id = ?", p.ID).Error; err != nil {
		// Create 路径：p 还未落库前的快照直接用入参
		full = *p
		full.ID = p.ID
	}
	return store.DB().Create(&PromptVersion{
		PromptID:    full.ID,
		Version:     version,
		Name:        full.Name,
		Category:    full.Category,
		Description: full.Description,
		Content:     full.Content,
		Variables:   full.Variables,
		ModelAlias:  full.ModelAlias,
		ChangeNote:  changeNote,
	}).Error
}

// ListVersions 列出模板的全部历史版本（按版本号倒序）。
func ListVersions(promptID string) (list []*PromptVersion, err error) {
	err = store.DB().Where("prompt_id = ?", promptID).
		Order("version desc").Find(&list).Error
	return
}

// GetVersion 取模板指定历史版本（version<=0 取最新）。
func GetVersion(promptID string, version int) (*PromptVersion, error) {
	var v PromptVersion
	db := store.DB().Where("prompt_id = ?", promptID)
	if version > 0 {
		db = db.Where("version = ?", version)
	} else {
		db = db.Order("version desc")
	}
	if err := db.First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// Rollback 把模板回滚到指定历史版本（产生一个新版本号，内容=历史版本）。
func Rollback(promptID string, targetVersion int) error {
	var hist PromptVersion
	if err := store.DB().First(&hist, "prompt_id = ? AND version = ?", promptID, targetVersion).Error; err != nil {
		return err
	}
	return UpdateWithNote(&PromptTemplate{
		Model:      commonmodel.Model{ID: promptID},
		Name:       hist.Name,
		Category:   hist.Category,
		Description: hist.Description,
		Content:    hist.Content,
		Variables:  hist.Variables,
		ModelAlias: hist.ModelAlias,
	}, fmt.Sprintf("rollback to v%d", targetVersion))
}

// Delete 删除模板（连同历史版本）。
func Delete(id string) error {
	_ = store.DB().Delete(&PromptVersion{}, "prompt_id = ?", id).Error
	return store.DB().Delete(&PromptTemplate{}, "id = ?", id).Error
}

// GetByID 按 ID 取模板。
func GetByID(id string) (*PromptTemplate, error) {
	var p PromptTemplate
	if err := store.DB().First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// List 列出租户模板（空 category 表示全部）。
func List(tenantID, category string) (list []*PromptTemplate, err error) {
	db := store.DB().Where("tenant_id IN (?, '')", tenantID)
	if category != "" {
		db = db.Where("category = ?", category)
	}
	err = db.Order("updated_at desc").Find(&list).Error
	return
}

// Render 安全渲染：把 Content 中的 {{var}} 替换为 vars[var]。
// 使用白名单替换而非 text/template，避免模板注入；未提供的变量替换为空串。
var tokenRe = regexp.MustCompile(`{{\s*([A-Za-z_][\w.-]*)\s*}}`)

func Render(content string, vars map[string]string) string {
	return tokenRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := tokenRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v, ok := vars[sub[1]]; ok {
			return v
		}
		return ""
	})
}

// ExtractVariables 扫描 Content，返回其中出现的变量名（有序、去重）。
func ExtractVariables(content string) []string {
	matches := tokenRe.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		if len(m) >= 2 && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// normalizeVariables 优先以显式传入的 Variables 为准，否则从 Content 扫描。
func normalizeVariables(content, declared string) string {
	if strings.TrimSpace(declared) != "" {
		return declared
	}
	vs := ExtractVariables(content)
	if len(vs) == 0 {
		return ""
	}
	sort.Strings(vs)
	return strings.Join(vs, ",")
}
