package conversation

import (
	"context"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
)

// Session represents a conversation thread
type Session struct {
	commonmodel.Model
	TenantID     string `json:"tenantID" gorm:"index;size:36"`
	PrincipalID  string `json:"principalID" gorm:"index;size:36"`
	ModelAlias   string `json:"modelAlias" gorm:"size:100"`
	Title        string `json:"title" gorm:"size:200"`
	MessageCount int    `json:"messageCount"`
	TotalTokens  int64  `json:"totalTokens"`
}

func (Session) TableName() string { return "conversation_session" }

// Message represents a single message in a session
type Message struct {
	commonmodel.Model
	SessionID string `json:"sessionID" gorm:"index;size:36"`
	Role      string `json:"role" gorm:"size:20;index"` // system / user / assistant / tool
	Content   string `json:"content" gorm:"type:text"`
	Tokens    int64  `json:"tokens"`
	ModelName string `json:"modelName" gorm:"size:100"`
}

func (Message) TableName() string { return "conversation_message" }

// CreateSession creates a new conversation session
func CreateSession(s *Session) (string, error) {
	if store.DB() == nil {
		return "", nil
	}
	if err := store.DB().Create(s).Error; err != nil {
		log.Errorf(context.Background(), "create session failed: %v", err)
		return "", err
	}
	return s.ID, nil
}

// GetSession returns a session by ID
func GetSession(id string) (*Session, error) {
	if store.DB() == nil {
		return nil, nil
	}
	var s Session
	if err := store.DB().First(&s, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSessions returns sessions for a tenant/principal with pagination
func ListSessions(tenantID, principalID string, limit, offset int) ([]Session, int64, error) {
	if store.DB() == nil {
		return nil, 0, nil
	}
	db := store.DB().Model(&Session{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if principalID != "" {
		db = db.Where("principal_id = ?", principalID)
	}
	var total int64
	db.Count(&total)
	var sessions []Session
	err := db.Order("updated_at desc").Limit(limit).Offset(offset).Find(&sessions).Error
	return sessions, total, err
}

// UpdateSessionTitle updates a session title
func UpdateSessionTitle(id, title string) error {
	if store.DB() == nil {
		return nil
	}
	return store.DB().Model(&Session{}).Where("id = ?", id).Update("title", title).Error
}

// DeleteSession deletes a session and all its messages
func DeleteSession(id string) error {
	if store.DB() == nil {
		return nil
	}
	if err := store.DB().Delete(&Message{}, "session_id = ?", id).Error; err != nil {
		log.Errorf(context.Background(), "delete session messages failed: %v", err)
	}
	return store.DB().Delete(&Session{}, "id = ?", id).Error
}

// AppendMessage appends a message to a session and updates session counters
func AppendMessage(m *Message) error {
	if store.DB() == nil {
		return nil
	}
	if err := store.DB().Create(m).Error; err != nil {
		log.Errorf(context.Background(), "append message failed: %v", err)
		return err
	}
	// Update session counters
	updates := map[string]interface{}{
		"message_count": store.DB().Model(&Message{}).Where("session_id = ?", m.SessionID).Select("count(*)"),
		"total_tokens":  store.DB().Model(&Message{}).Where("session_id = ?", m.SessionID).Select("COALESCE(SUM(tokens),0)"),
		"updated_at":    time.Now(),
	}
	return store.DB().Model(&Session{}).Where("id = ?", m.SessionID).Updates(updates).Error
}

// GetMessages returns the last N messages in a session, ordered by time ASC
func GetMessages(sessionID string, limit int) ([]Message, error) {
	if store.DB() == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var messages []Message
	err := store.DB().Where("session_id = ?", sessionID).
		Order("created_at desc").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, err
	}
	// Reverse to chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// CountMessages returns the total number of messages in a session
func CountMessages(sessionID string) (int64, error) {
	if store.DB() == nil {
		return 0, nil
	}
	var count int64
	err := store.DB().Model(&Message{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}
