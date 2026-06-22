// Package prompt 提供 Prompt 模板管理：按租户存储可复用 Prompt，支持 {{变量}} 渲染。
//
// 配合 AI 网关使用：应用拉取模板 → 渲染变量 → 调 /v1/chat/completions，
// 把"Prompt 工程"与"模型调用"分离，模板版本化集中管理。
package prompt

import (
	"regexp"
	"sort"
	"strings"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/store"
)

// PromptTemplate 可复用 Prompt 模板。
type PromptTemplate struct {
	commonmodel.Model
	TenantID    string `json:"tenantID" gorm:"index;size:36"`
	Name        string `json:"name" gorm:"size:100;comment:模板名称"`
	Category    string `json:"category" gorm:"size:50;index;comment:分类(system/agent/rag等)"`
	Description string `json:"description" gorm:"size:500"`
	Content     string `json:"content" gorm:"type:text;comment:模板正文，含 {{变量}} 占位"`
	Variables   string `json:"variables" gorm:"size:500;comment:逗号分隔的变量名清单"`
	ModelAlias  string `json:"modelAlias" gorm:"size:100;comment:建议模型别名"`
	Enable      bool   `json:"enable" gorm:"index;default:true"`
}

func (PromptTemplate) TableName() string { return "prompt_template" }

// Create 创建模板。
func Create(p *PromptTemplate) (string, error) {
	p.Variables = normalizeVariables(p.Content, p.Variables)
	if err := store.DB().Create(p).Error; err != nil {
		return "", err
	}
	return p.ID, nil
}

// Update 更新模板。
func Update(p *PromptTemplate) error {
	p.Variables = normalizeVariables(p.Content, p.Variables)
	return store.DB().Model(&PromptTemplate{}).Where("id = ?", p.ID).
		Updates(map[string]interface{}{
			"name": p.Name, "category": p.Category, "description": p.Description,
			"content": p.Content, "variables": p.Variables, "model_alias": p.ModelAlias, "enable": p.Enable,
		}).Error
}

// Delete 删除模板。
func Delete(id string) error {
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
