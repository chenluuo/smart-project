package agent

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateSession(ctx context.Context, session *Session) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if session.PlotID != nil {
			var count int64
			if err := tx.Table("plots").Where("id = ? AND owner_id = ?", *session.PlotID, session.UserID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrNotFound
			}
		}
		return tx.Create(session).Error
	})
}

func (r *Repository) AppendMessage(ctx context.Context, message *Message) error {
	return r.appendMessage(ctx, message, 0)
}

func (r *Repository) AppendMessageByOwner(ctx context.Context, message *Message, userID uint64) error {
	if userID == 0 {
		return ErrInvalidInput
	}
	return r.appendMessage(ctx, message, userID)
}

func (r *Repository) appendMessage(ctx context.Context, message *Message, userID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session Session
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", message.SessionID)
		if userID != 0 {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.Take(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if session.Status != SessionStatusActive {
			return ErrConflict
		}
		if message.PlotID != nil && (session.PlotID == nil || *message.PlotID != *session.PlotID) {
			return ErrInvalidInput
		}
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		return tx.Model(&Session{}).Where("id = ?", session.ID).Updates(map[string]any{
			"last_message_at": message.CreatedAt,
			"updated_at":      message.CreatedAt,
		}).Error
	})
}

func (r *Repository) ListMessagesByOwner(ctx context.Context, sessionID string, userID uint64, page, pageSize int) ([]Message, int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Session{}).Where("id = ? AND user_id = ?", sessionID, userID).Count(&count).Error; err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return nil, 0, ErrNotFound
	}
	query := r.db.WithContext(ctx).Model(&Message{}).Where("session_id = ?", sessionID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var messages []Message
	err := query.Order("created_at ASC, id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&messages).Error
	return messages, total, err
}

func (r *Repository) CloseSessionByOwner(ctx context.Context, sessionID string, userID uint64, now time.Time) (*Session, error) {
	var session Session
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", sessionID, userID).Take(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if session.Status == SessionStatusClosed {
			return nil
		}
		session.Status = SessionStatusClosed
		session.ClosedAt = &now
		session.UpdatedAt = now
		return tx.Model(&Session{}).Where("id = ?", session.ID).Updates(map[string]any{
			"status": SessionStatusClosed, "closed_at": now, "updated_at": now,
		}).Error
	})
	return &session, err
}

// TokenUsage 聚合当前用户在 chat_messages 的 ASSISTANT 消息 token 消耗（今日 + 累计）。
func (r *Repository) TokenUsage(ctx context.Context, userID uint64) (TokenUsage, error) {
	var usage TokenUsage
	query := r.db.WithContext(ctx).Table("chat_messages AS m").
		Joins("JOIN chat_sessions AS s ON s.id = m.session_id").
		Where("s.user_id = ? AND m.role = ?", userID, MessageRoleAssistant)

	row := query.Select(`
		COALESCE(SUM(m.prompt_tokens), 0)     AS total_prompt,
		COALESCE(SUM(m.completion_tokens), 0) AS total_completion,
		COALESCE(SUM(CASE WHEN m.created_at >= CURDATE() THEN m.prompt_tokens ELSE 0 END), 0)     AS today_prompt,
		COALESCE(SUM(CASE WHEN m.created_at >= CURDATE() THEN m.completion_tokens ELSE 0 END), 0) AS today_completion`).
		Row()
	if err := row.Scan(&usage.TotalPromptTokens, &usage.TotalCompletionTokens, &usage.TodayPromptTokens, &usage.TodayCompletionTokens); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return usage, nil
		}
		return usage, err
	}
	usage.Total = usage.TotalPromptTokens + usage.TotalCompletionTokens
	usage.TodayTotal = usage.TodayPromptTokens + usage.TodayCompletionTokens
	return usage, nil
}
