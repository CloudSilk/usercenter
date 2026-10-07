package server

import (
	"github.com/CloudSilk/usercenter/internal/prompt"
	"gorm.io/gorm"
)

// PromptTemplate and PromptVersion are UserCenter-owned storage models. Hosts
// keep references to their IDs, rather than creating competing prompt tables.
type PromptTemplate = prompt.PromptTemplate
type PromptVersion = prompt.PromptVersion

// Prompts binds prompt operations to the host's DB/transaction and tenant.
// The caller supplies its authenticated tenant. Empty tenant explicitly selects
// platform templates. Tenant callers may read platform templates but not edit them.
func Prompts(db *gorm.DB, tenantID string) *prompt.Repository {
	return prompt.NewRepository(db).ForTenant(tenantID)
}

func RenderPrompt(content string, variables map[string]string) string {
	return prompt.Render(content, variables)
}
func PromptVariables(content string) []string { return prompt.ExtractVariables(content) }
