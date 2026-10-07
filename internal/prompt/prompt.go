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

// Create creates a template and its initial immutable snapshot atomically.
func Create(p *PromptTemplate) (string, error) { return NewRepository(store.DB()).Create(p) }
func Update(p *PromptTemplate) error           { return UpdateWithNote(p, "") }
func UpdateWithNote(p *PromptTemplate, note string) error {
	return NewRepository(store.DB()).UpdateWithNote(p, note)
}
func ListVersions(id string) ([]*PromptVersion, error) {
	return NewRepository(store.DB()).ListVersions(id)
}
func GetVersion(id string, version int) (*PromptVersion, error) {
	return NewRepository(store.DB()).GetVersion(id, version)
}
func Rollback(id string, version int) error      { return NewRepository(store.DB()).Rollback(id, version) }
func Delete(id string) error                     { return NewRepository(store.DB()).Delete(id) }
func GetByID(id string) (*PromptTemplate, error) { return NewRepository(store.DB()).GetByID(id) }
func List(tenantID, category string) ([]*PromptTemplate, error) {
	return NewRepository(store.DB()).ForTenant(tenantID).List(category)
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
