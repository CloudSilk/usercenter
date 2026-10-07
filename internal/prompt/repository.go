package prompt

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository supports the embedded host's transaction/connection. A scoped
// repository reads its tenant plus platform templates and writes only its tenant.
type Repository struct {
	db     *gorm.DB
	tenant *string
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) ForTenant(tenant string) *Repository {
	return &Repository{db: r.db, tenant: &tenant}
}
func (r *Repository) on(db *gorm.DB) *Repository { return &Repository{db: db, tenant: r.tenant} }
func (r *Repository) query(write bool) *gorm.DB {
	q := r.db.Model(&PromptTemplate{})
	if r.tenant != nil {
		if write {
			q = q.Where("tenant_id = ?", *r.tenant)
		} else {
			q = q.Where("tenant_id IN ?", []string{*r.tenant, ""})
		}
	}
	return q
}

func (r *Repository) Create(p *PromptTemplate) (string, error) {
	copyOfPrompt := *p
	if r.tenant != nil {
		if copyOfPrompt.TenantID != "" && copyOfPrompt.TenantID != *r.tenant {
			return "", fmt.Errorf("prompt tenant does not match repository")
		}
		copyOfPrompt.TenantID = *r.tenant
	}
	copyOfPrompt.Variables = normalizeVariables(copyOfPrompt.Content, copyOfPrompt.Variables)
	copyOfPrompt.Version = 1
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&copyOfPrompt).Error; err != nil {
			return err
		}
		_, err := snapshotVersionTx(tx, &copyOfPrompt, "")
		return err
	})
	if err != nil {
		return "", err
	}
	*p = copyOfPrompt
	return p.ID, nil
}

func (r *Repository) UpdateWithNote(p *PromptTemplate, note string) error {
	copyOfPrompt := *p
	copyOfPrompt.Variables = normalizeVariables(copyOfPrompt.Content, copyOfPrompt.Variables)
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var current PromptTemplate
		if err := r.on(tx).query(true).Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", p.ID).Error; err != nil {
			return err
		}
		// Repair only a missing snapshot of the current state from older writers.
		// Never rewrite a historical version or manufacture missing older content.
		if _, err := snapshotVersionTx(tx, &current, ""); err != nil {
			return err
		}
		if p.Version > 0 && p.Version != current.Version {
			return fmt.Errorf("prompt version conflict: expected %d, current %d", p.Version, current.Version)
		}
		copyOfPrompt.Version = current.Version + 1
		copyOfPrompt.TenantID = current.TenantID
		result := r.on(tx).query(true).Where("id = ? AND version = ?", p.ID, current.Version).Updates(map[string]any{
			"name": copyOfPrompt.Name, "category": copyOfPrompt.Category, "description": copyOfPrompt.Description,
			"content": copyOfPrompt.Content, "variables": copyOfPrompt.Variables, "model_alias": copyOfPrompt.ModelAlias,
			"enable": copyOfPrompt.Enable, "version": copyOfPrompt.Version,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("prompt version changed during update")
		}
		_, err := snapshotVersionTx(tx, &copyOfPrompt, note)
		return err
	})
	if err == nil {
		*p = copyOfPrompt
	}
	return err
}

// snapshotVersionTx is idempotent for an existing identical snapshot. Older
// releases could create duplicates; retain that evidence, without adding more.
func snapshotVersionTx(tx *gorm.DB, p *PromptTemplate, note string) (*PromptVersion, error) {
	var existing []PromptVersion
	if err := tx.Where("prompt_id = ? AND version = ?", p.ID, p.Version).Order("created_at ASC, id ASC").Find(&existing).Error; err != nil {
		return nil, err
	}
	for _, v := range existing {
		if v.Name != p.Name || v.Category != p.Category || v.Description != p.Description || v.Content != p.Content || v.Variables != p.Variables || v.ModelAlias != p.ModelAlias {
			return nil, fmt.Errorf("prompt version %d has conflicting historical content", p.Version)
		}
	}
	if len(existing) > 0 {
		return &existing[0], nil
	}
	v := &PromptVersion{PromptID: p.ID, Version: p.Version, Name: p.Name, Category: p.Category, Description: p.Description, Content: p.Content, Variables: p.Variables, ModelAlias: p.ModelAlias, ChangeNote: note}
	if err := tx.Create(v).Error; err != nil {
		return nil, err
	}
	return v, nil
}

func (r *Repository) GetByID(id string) (*PromptTemplate, error) {
	var p PromptTemplate
	if err := r.query(false).First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) List(category string) (list []*PromptTemplate, err error) {
	q := r.query(false)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	err = q.Order("updated_at DESC").Find(&list).Error
	return
}

func (r *Repository) ListVersions(id string) (list []*PromptVersion, err error) {
	if _, err = r.GetByID(id); err != nil {
		return nil, err
	}
	err = r.db.Where("prompt_id = ?", id).Order("version DESC, created_at ASC, id ASC").Find(&list).Error
	return
}

func (r *Repository) GetVersion(id string, version int) (*PromptVersion, error) {
	p, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	if version <= 0 {
		version = p.Version
	}
	var v PromptVersion
	if err := r.db.Where("prompt_id = ? AND version = ?", id, version).Order("created_at ASC, id ASC").First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// PinCurrent returns a durable, immutable version reference for a host task.
// An old current row missing its snapshot is repaired atomically while locked.
func (r *Repository) PinCurrent(id string) (*PromptTemplate, *PromptVersion, error) {
	return r.snapshotCurrent(id, true)
}

// SnapshotCurrent preserves a known current state even when disabled. It is
// intended for archival migrations; task enqueueing must use PinCurrent.
func (r *Repository) SnapshotCurrent(id string) (*PromptTemplate, *PromptVersion, error) {
	return r.snapshotCurrent(id, false)
}

func (r *Repository) snapshotCurrent(id string, requireEnabled bool) (*PromptTemplate, *PromptVersion, error) {
	var p PromptTemplate
	var v *PromptVersion
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.on(tx).query(false).Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, "id = ?", id).Error; err != nil {
			return err
		}
		if requireEnabled && !p.Enable {
			return fmt.Errorf("prompt template is disabled")
		}
		var err error
		v, err = snapshotVersionTx(tx, &p, "")
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &p, v, nil
}

func (r *Repository) GetVersionByID(promptID, versionID string) (*PromptVersion, error) {
	if _, err := r.GetByID(promptID); err != nil {
		return nil, err
	}
	var v PromptVersion
	if err := r.db.First(&v, "id = ? AND prompt_id = ?", versionID, promptID).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *Repository) Rollback(id string, version int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		inner := r.on(tx)
		var current PromptTemplate
		if err := inner.query(true).Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", id).Error; err != nil {
			return err
		}
		history, err := inner.GetVersion(id, version)
		if err != nil {
			return err
		}
		current.Name, current.Category, current.Description = history.Name, history.Category, history.Description
		current.Content, current.Variables, current.ModelAlias = history.Content, history.Variables, history.ModelAlias
		return inner.UpdateWithNote(&current, fmt.Sprintf("rollback to v%d", version))
	})
}

func (r *Repository) Delete(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		result := r.on(tx).query(true).Where("id = ?", id).Delete(&PromptTemplate{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Delete(&PromptVersion{}, "prompt_id = ?", id).Error
	})
}
