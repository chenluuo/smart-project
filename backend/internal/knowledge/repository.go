package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/audit"
	"github.com/chenluuo/smart-project/backend/internal/outbox"
	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListActive(ctx context.Context, category string) ([]Document, error) {
	query := r.db.WithContext(ctx).Where("status = ?", StatusActive)
	if category != "" {
		query = query.Where("category = ?", category)
	}
	var documents []Document
	err := query.Order("category ASC, title ASC, version DESC").Find(&documents).Error
	return documents, err
}

// DocumentWithUploader 文档 + 上传人用户名（管理后台列表用）。
type DocumentWithUploader struct {
	Document
	UploaderName string
}

// ListAll 管理后台全状态文档列表（status 为空表示全部状态）。
func (r *Repository) ListAll(ctx context.Context, filter AdminListFilter) ([]DocumentWithUploader, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	keyword := strings.TrimSpace(filter.Keyword)
	category := strings.TrimSpace(filter.Category)

	applyFilter := func(query *gorm.DB) *gorm.DB {
		if filter.Status != nil {
			query = query.Where("d.status = ?", *filter.Status)
		}
		if category != "" {
			query = query.Where("d.category = ?", category)
		}
		if keyword != "" {
			query = query.Where("d.title LIKE ?", "%"+keyword+"%")
		}
		return query
	}

	countQuery := r.db.WithContext(ctx).Table("knowledge_documents d")
	countQuery = applyFilter(countQuery)
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	listQuery := r.db.WithContext(ctx).Table("knowledge_documents d").
		Select("d.*, COALESCE(u.name, '') AS uploader_name").
		Joins("LEFT JOIN users u ON u.id = d.uploaded_by")
	listQuery = applyFilter(listQuery)
	var rows []DocumentWithUploader
	if err := listQuery.Order("d.id DESC").Limit(filter.PageSize).Offset((filter.Page - 1) * filter.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) Create(ctx context.Context, document *Document, actorID uint64, traceID string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(document).Error; err != nil {
			return err
		}
		return createSideEffects(tx, document, actorID, traceID, "KNOWLEDGE_DOCUMENT_REGISTER", "UPLOADED", document.CreatedAt)
	})
	return normalizeRepositoryError(err)
}

func (r *Repository) Transition(ctx context.Context, documentID, actorID uint64, target Status, traceID string, now time.Time) (*Document, error) {
	var document Document
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&document, documentID).Error; err != nil {
			return err
		}
		if !validTransition(document.Status, target) {
			return ErrConflict
		}

		updates := map[string]any{"status": target, "updated_at": now}
		event := "UPDATED"
		action := "KNOWLEDGE_DOCUMENT_" + string(target)
		switch target {
		case StatusApproved:
			updates["approved_by"] = actorID
			document.ApprovedBy = &actorID
		case StatusActive:
			updates["published_at"] = now
			document.PublishedAt = &now
			var previousActive []Document
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("title = ? AND status = ? AND id <> ?", document.Title, StatusActive, document.ID).
				Find(&previousActive).Error; err != nil {
				return err
			}
			for index := range previousActive {
				previousActive[index].Status = StatusArchived
				previousActive[index].ArchivedAt = &now
				previousActive[index].UpdatedAt = now
				if err := tx.Model(&Document{}).Where("id = ?", previousActive[index].ID).
					Updates(map[string]any{"status": StatusArchived, "archived_at": now, "updated_at": now}).Error; err != nil {
					return err
				}
				if err := createSideEffects(tx, &previousActive[index], actorID, traceID,
					"KNOWLEDGE_DOCUMENT_ARCHIVED", "ARCHIVED", now); err != nil {
					return err
				}
			}
		case StatusArchived:
			updates["archived_at"] = now
			document.ArchivedAt = &now
			event = "ARCHIVED"
		}
		if err := tx.Model(&Document{}).Where("id = ?", document.ID).Updates(updates).Error; err != nil {
			return err
		}
		document.Status = target
		document.UpdatedAt = now
		return createSideEffects(tx, &document, actorID, traceID, action, event, now)
	})
	return &document, normalizeRepositoryError(err)
}

// Delete 物理删除文档：删 DB 行 + 写审计日志 + 写 outbox DELETED 事件（通知 agent 清理向量）。
// 返回被删文档（含 objectKey，供调用方清理对象存储）。
func (r *Repository) Delete(ctx context.Context, documentID, actorID uint64, traceID string) (*Document, error) {
	var document Document
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&document, documentID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&Document{}, documentID).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		document.UpdatedAt = now
		return createSideEffects(tx, &document, actorID, traceID, "KNOWLEDGE_DOCUMENT_DELETED", "DELETED", now)
	})
	return &document, normalizeRepositoryError(err)
}

func createSideEffects(tx *gorm.DB, document *Document, actorID uint64, traceID, action, event string, now time.Time) error {
	resourceID := strconv.FormatUint(document.ID, 10)
	var traceIDPointer *string
	if traceID != "" {
		traceIDPointer = &traceID
	}
	log := audit.Log{
		ActorID: &actorID, Action: action, ResourceType: "knowledge_document", ResourceID: &resourceID,
		Result: "SUCCESS", TraceID: traceIDPointer,
		Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
	}
	if err := tx.Create(&log).Error; err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"docId": document.ID, "event": event, "version": document.Version,
		"objectKey": document.ObjectKey, "traceId": traceID,
	})
	if err != nil {
		return err
	}
	outboxEvent := outbox.Event{
		AggregateType: "knowledge_document", AggregateID: resourceID,
		EventType: "KNOWLEDGE_DOCUMENT_" + event, Payload: datatypes.JSON(payload),
		Status: outbox.StatusPending, AvailableAt: now,
		Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
	}
	return tx.Create(&outboxEvent).Error
}

func validTransition(current, target Status) bool {
	switch current {
	case StatusDraft:
		return target == StatusApproved
	case StatusApproved:
		return target == StatusActive
	case StatusActive:
		return target == StatusArchived
	default:
		return false
	}
}

func normalizeRepositoryError(err error) error {
	if err == nil || errors.Is(err, ErrConflict) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var mysqlErr *drivermysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrConflict
	}
	return err
}
