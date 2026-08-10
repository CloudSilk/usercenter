package server

import (
	"errors"
	"strings"
	"time"

	uctenant "github.com/CloudSilk/usercenter/internal/tenant"
)

type TenantRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Contact   string    `json:"contact"`
	CellPhone string    `json:"cellPhone"`
	Address   string    `json:"address"`
	Enabled   bool      `json:"enabled"`
	UserLimit int32     `json:"userLimit"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type CreateTenantRequest struct {
	ID        string
	Name      string
	Contact   string
	CellPhone string
	Address   string
	Enabled   bool
	UserLimit int32
	ExpiresAt time.Time
}

type UpdateTenantRequest struct {
	ID        string
	Name      string
	Contact   string
	CellPhone string
	Address   string
	UserLimit int32
	ExpiresAt time.Time
}

func CreateTenant(request CreateTenantRequest) (TenantRecord, error) {
	request.ID = strings.TrimSpace(request.ID)
	request.Name = strings.TrimSpace(request.Name)
	if request.ID == "" || request.Name == "" {
		return TenantRecord{}, errors.New("tenant ID and name are required")
	}
	if request.UserLimit < 1 {
		return TenantRecord{}, errors.New("tenant user limit must be positive")
	}
	if request.ExpiresAt.IsZero() {
		return TenantRecord{}, errors.New("tenant expiration is required")
	}

	tenant := &uctenant.Tenant{
		Name: request.Name, Contact: strings.TrimSpace(request.Contact),
		CellPhone: strings.TrimSpace(request.CellPhone), Address: strings.TrimSpace(request.Address),
		Enable: request.Enabled, UserCount: request.UserLimit, Expired: request.ExpiresAt,
	}
	tenant.ID = request.ID
	if err := uctenant.CreateTenant(tenant); err != nil {
		return TenantRecord{}, err
	}
	return tenantRecord(tenant), nil
}

func GetTenant(id string) (TenantRecord, error) {
	tenant, err := uctenant.GetTenantByID(strings.TrimSpace(id))
	if err != nil {
		return TenantRecord{}, err
	}
	return tenantRecord(tenant), nil
}

func UpdateTenant(request UpdateTenantRequest) (TenantRecord, error) {
	current, err := uctenant.GetTenantByID(strings.TrimSpace(request.ID))
	if err != nil {
		return TenantRecord{}, err
	}
	if name := strings.TrimSpace(request.Name); name != "" {
		current.Name = name
	}
	current.Contact = strings.TrimSpace(request.Contact)
	current.CellPhone = strings.TrimSpace(request.CellPhone)
	current.Address = strings.TrimSpace(request.Address)
	if request.UserLimit > 0 {
		current.UserCount = request.UserLimit
	}
	if !request.ExpiresAt.IsZero() {
		current.Expired = request.ExpiresAt
	}
	if err := uctenant.UpdateTenant(current); err != nil {
		return TenantRecord{}, err
	}
	return tenantRecord(current), nil
}

func SetTenantEnabled(id string, enabled bool) error {
	return uctenant.EnableTenant(strings.TrimSpace(id), enabled)
}

func tenantRecord(tenant *uctenant.Tenant) TenantRecord {
	return TenantRecord{
		ID: tenant.ID, Name: tenant.Name, Contact: tenant.Contact, CellPhone: tenant.CellPhone,
		Address: tenant.Address, Enabled: tenant.Enable, UserLimit: tenant.UserCount, ExpiresAt: tenant.Expired,
	}
}
