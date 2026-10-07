package prompt

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func promptTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "prompts.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&PromptTemplate{}, &PromptVersion{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestRepositoryHistoryMatchesCurrentAndRollbackPreservesEnable(t *testing.T) {
	db := promptTestDB(t)
	repo := NewRepository(db).ForTenant("tenant-a")
	p := &PromptTemplate{Name: "draft", Content: "v1 {{name}}", Enable: true}
	if _, err := repo.Create(p); err != nil {
		t.Fatal(err)
	}
	first, err := repo.GetVersion(p.ID, 0)
	if err != nil || first.Version != 1 || first.Content != p.Content {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	p.Content, p.Variables = "v2 {{title}}", ""
	if err := repo.UpdateWithNote(p, "second draft"); err != nil {
		t.Fatal(err)
	}
	latest, err := repo.GetVersion(p.ID, 0)
	if err != nil || latest.Version != 2 || latest.Content != p.Content || latest.Variables != "title" || latest.ChangeNote != "second draft" {
		t.Fatalf("latest history lagged current: %+v err=%v", latest, err)
	}
	if err := repo.Rollback(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	current, err := repo.GetByID(p.ID)
	if err != nil || current.Version != 3 || current.Content != first.Content || !current.Enable || current.TenantID != "tenant-a" {
		t.Fatalf("rollback=%+v err=%v", current, err)
	}
	versions, err := repo.ListVersions(p.ID)
	if err != nil || len(versions) != 3 || versions[2].ID != first.ID {
		t.Fatalf("history overwritten/duplicated: %+v err=%v", versions, err)
	}
	// Stale callers cannot replace a newer revision.
	p.Content = "stale update"
	if err := repo.UpdateWithNote(p, ""); err == nil {
		t.Fatal("stale version accepted")
	}
	current, err = repo.GetByID(p.ID)
	if err != nil || current.Version != 3 || current.Content != first.Content {
		t.Fatalf("stale writer changed current: %+v %v", current, err)
	}
}

func TestRepositorySnapshotsAndCurrentRollBackTogether(t *testing.T) {
	db := promptTestDB(t)
	repo := NewRepository(db).ForTenant("tenant-a")
	failSnapshot := true
	if err := db.Callback().Create().Before("gorm:create").Register("test:fail-snapshot", func(tx *gorm.DB) {
		if failSnapshot && tx.Statement.Table == "prompt_template_version" {
			tx.AddError(errors.New("snapshot unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	p := &PromptTemplate{Name: "draft", Content: "original", Enable: true}
	if _, err := repo.Create(p); err == nil {
		t.Fatal("create ignored snapshot failure")
	}
	var count int64
	if err := db.Model(&PromptTemplate{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("partial template survived: %d %v", count, err)
	}
	failSnapshot = false
	if _, err := repo.Create(p); err != nil {
		t.Fatal(err)
	}
	p.Content = "new draft"
	failSnapshot = true
	if err := repo.UpdateWithNote(p, ""); err == nil {
		t.Fatal("update ignored snapshot failure")
	}
	current, err := repo.GetByID(p.ID)
	if err != nil || current.Content != "original" || current.Version != 1 || p.Version != 1 {
		t.Fatalf("partial update survived: %+v %v", current, err)
	}
	versions, err := repo.ListVersions(p.ID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("partial history: %+v %v", versions, err)
	}
}

func TestRepositoryScopesAndPinsImmutableVersion(t *testing.T) {
	db := promptTestDB(t)
	global := NewRepository(db).ForTenant("")
	shared := &PromptTemplate{Name: "shared", Content: "shared content", Enable: true}
	if _, err := global.Create(shared); err != nil {
		t.Fatal(err)
	}
	a := NewRepository(db).ForTenant("a")
	b := NewRepository(db).ForTenant("b")
	p := &PromptTemplate{Name: "private", Content: "original", Enable: true}
	if _, err := a.Create(p); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetByID(p.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign read=%v", err)
	}
	if _, _, err := b.PinCurrent(p.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign pin=%v", err)
	}
	if _, err := b.ListVersions(p.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign history=%v", err)
	}
	if err := b.Delete(p.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign delete=%v", err)
	}
	if err := a.UpdateWithNote(shared, ""); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant updated global prompt: %v", err)
	}
	if _, err := a.GetByID(shared.ID); err != nil {
		t.Fatal(err)
	}
	_, pinned, err := a.PinCurrent(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	p.Content = "new content"
	if err := a.UpdateWithNote(p, ""); err != nil {
		t.Fatal(err)
	}
	old, err := a.GetVersionByID(p.ID, pinned.ID)
	if err != nil || old.Content != "original" {
		t.Fatalf("pin changed: %+v %v", old, err)
	}
	if _, err := a.GetVersionByID(shared.ID, pinned.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-template version accepted: %v", err)
	}
}

func TestRepositoryRepairsMissingCurrentSnapshotWithoutInventingHistory(t *testing.T) {
	db := promptTestDB(t)
	p := &PromptTemplate{TenantID: "a", Name: "legacy", Content: "known current content", Enable: true, Version: 4}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db).ForTenant("a")
	_, pin, err := repo.PinCurrent(p.ID)
	if err != nil || pin.Version != 4 || pin.Content != p.Content {
		t.Fatalf("pin=%+v %v", pin, err)
	}
	_, again, err := repo.PinCurrent(p.ID)
	if err != nil || pin.ID != again.ID {
		t.Fatalf("pin duplicated: %+v %v", again, err)
	}
	versions, err := repo.ListVersions(p.ID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("invented versions: %+v %v", versions, err)
	}
	if err := db.Model(&PromptTemplate{}).Where("id = ?", p.ID).Update("content", "unversioned mutation").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.PinCurrent(p.ID); err == nil {
		t.Fatal("conflicting historical snapshot silently reused")
	}
}
