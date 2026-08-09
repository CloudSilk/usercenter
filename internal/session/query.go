package session

import (
	"strings"
	"time"

	"github.com/CloudSilk/usercenter/internal/store"
)

const (
	SessionStatusActive     = "ACTIVE"
	SessionStatusIdle       = "IDLE"
	SessionStatusTerminated = "TERMINATED"
)

type Query struct {
	PrincipalID string
	TenantID    string
	Keyword     string
	ActiveOnly  bool
	PageIndex   int64
	PageSize    int64
	IdleMinutes int
}

type View struct {
	Session
	UserName    string `json:"userName"`
	DisplayName string `json:"displayName"`
	TenantName  string `json:"tenantName"`
	Status      string `json:"status"`
}

func QuerySessions(query Query) (list []*View, total int64, err error) {
	database := store.DB()
	if database == nil {
		return nil, 0, errStoreUnavailable
	}

	db := database.Model(&Session{})
	if query.PrincipalID != "" {
		db = db.Where("principal_id = ?", query.PrincipalID)
	}
	if query.TenantID != "" {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.ActiveOnly {
		db = db.Where("revoked = ?", false)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		userTable := database.NamingStrategy.TableName("User")
		tenantTable := database.NamingStrategy.TableName("Tenant")
		userIDs := database.Table(userTable).
			Select("id").
			Where("user_name LIKE ? OR nickname LIKE ? OR real_name LIKE ?", like, like, like)
		tenantIDs := database.Table(tenantTable).Select("id").Where("name LIKE ?", like)
		db = db.Where(
			"principal_id IN (?) OR tenant_id IN (?) OR ip LIKE ? OR device_name LIKE ? OR user_agent LIKE ?",
			userIDs, tenantIDs, like, like, like,
		)
	}

	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	pageIndex, pageSize := normalizePage(query.PageIndex, query.PageSize)
	var sessions []*Session
	if err = db.Order("created_at desc").
		Offset(int((pageIndex - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	userNames, tenantNames, err := loadSessionNames(sessions)
	if err != nil {
		return nil, 0, err
	}
	idleMinutes := query.IdleMinutes
	if idleMinutes <= 0 {
		idleMinutes = 15
	}
	list = make([]*View, 0, len(sessions))
	for _, item := range sessions {
		userInfo := userNames[item.PrincipalID]
		list = append(list, &View{
			Session:     *item,
			UserName:    userInfo.UserName,
			DisplayName: userInfo.DisplayName,
			TenantName:  tenantNames[item.TenantID],
			Status:      sessionStatus(item, idleMinutes),
		})
	}
	return list, total, nil
}

var errStoreUnavailable = &storeError{message: "session store is not initialized"}

type storeError struct{ message string }

func (e *storeError) Error() string { return e.message }

type sessionUserName struct {
	UserName    string
	DisplayName string
}

func loadSessionNames(sessions []*Session) (map[string]sessionUserName, map[string]string, error) {
	users := make(map[string]sessionUserName)
	tenants := make(map[string]string)
	if len(sessions) == 0 {
		return users, tenants, nil
	}

	principalIDs := make([]string, 0, len(sessions))
	tenantIDs := make([]string, 0, len(sessions))
	for _, item := range sessions {
		principalIDs = append(principalIDs, item.PrincipalID)
		tenantIDs = append(tenantIDs, item.TenantID)
	}
	database := store.DB()
	userTable := database.NamingStrategy.TableName("User")
	var userRows []struct {
		ID       string `gorm:"column:id"`
		UserName string `gorm:"column:user_name"`
		Nickname string `gorm:"column:nickname"`
		RealName string `gorm:"column:real_name"`
	}
	if err := database.Table(userTable).
		Select("id, user_name, nickname, real_name").
		Where("id IN ?", principalIDs).
		Scan(&userRows).Error; err != nil {
		return nil, nil, err
	}
	for _, row := range userRows {
		displayName := strings.TrimSpace(row.RealName)
		if displayName == "" {
			displayName = strings.TrimSpace(row.Nickname)
		}
		if displayName == "" {
			displayName = row.UserName
		}
		users[row.ID] = sessionUserName{UserName: row.UserName, DisplayName: displayName}
	}

	tenantTable := database.NamingStrategy.TableName("Tenant")
	var tenantRows []struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := database.Table(tenantTable).
		Select("id, name").
		Where("id IN ?", tenantIDs).
		Scan(&tenantRows).Error; err != nil {
		return nil, nil, err
	}
	for _, row := range tenantRows {
		tenants[row.ID] = row.Name
	}
	return users, tenants, nil
}

func sessionStatus(item *Session, idleMinutes int) string {
	if item.Revoked {
		return SessionStatusTerminated
	}
	if item.LastActiveAt < time.Now().Add(-time.Duration(idleMinutes)*time.Minute).Unix() {
		return SessionStatusIdle
	}
	return SessionStatusActive
}

func normalizePage(pageIndex, pageSize int64) (int64, int64) {
	if pageIndex <= 0 {
		pageIndex = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return pageIndex, pageSize
}
