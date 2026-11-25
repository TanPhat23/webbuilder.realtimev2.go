package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	domainRepo "webbuilder/internal/domain/repository"
)

var elementMutableFields = []string{
	"Name", "Type", "Content", "Href", "Src",
	"Styles", "Settings", "TailwindStyles",
	"Order", "OrderKey", "IsLocked", "ParentID",
}

type elementRepository struct {
	db *gorm.DB
}

// NewElementRepository creates a new instance of ElementRepository
func NewElementRepository(db *gorm.DB) domainRepo.ElementRepository {
	return &elementRepository{db: db}
}

func pageBelongsToProject(tx *gorm.DB, projectID, pageID string) (bool, error) {
	if projectID == "" || pageID == "" {
		return false, nil
	}
	if tx.Error != nil {
		return false, tx.Error
	}
	var one int
	err := tx.Raw(`SELECT 1 FROM "Page" WHERE "Id" = ? AND "ProjectId" = ? AND "DeletedAt" IS NULL LIMIT 1`, pageID, projectID).Row().Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return one == 1, nil
}

func verifyPageOwnership(tx *gorm.DB, projectID, pageID string) error {
	ok, err := pageBelongsToProject(tx, projectID, pageID)
	if err != nil {
		return fmt.Errorf("failed to verify page ownership: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: page %s does not belong to project %s", domainErrors.ErrNotFound, pageID, projectID)
	}
	return nil
}

// PageBelongsToProject reports whether pageID belongs to projectID and is not soft-deleted.
func (r *elementRepository) PageBelongsToProject(ctx context.Context, projectID string, pageID string) (bool, error) {
	return pageBelongsToProject(r.db.WithContext(ctx), projectID, pageID)
}

// CreateElement persists a new element to the database, ensuring the page belongs to projectID.
func (r *elementRepository) CreateElement(ctx context.Context, projectID string, pageID string, element *domainEntity.Element) error {
	if element == nil {
		return fmt.Errorf("element is nil")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyPageOwnership(tx, projectID, pageID); err != nil {
			return err
		}
		element.PageID = pageID
		if err := tx.Create(element).Error; err != nil {
			return fmt.Errorf("failed to create element: %w", err)
		}
		return nil
	})
}

// CreateElements replaces elements for a page after verifying page ownership in the transaction.
func (r *elementRepository) CreateElements(ctx context.Context, projectID string, pageId string, elements []domainEntity.Element) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyPageOwnership(tx, projectID, pageId); err != nil {
			return err
		}

		if err := tx.Where(`"PageId" = ?`, pageId).Delete(&domainEntity.Element{}).Error; err != nil {
			return fmt.Errorf("failed to clear elements for page %s: %w", pageId, err)
		}

		for i := range elements {
			elements[i].PageID = pageId
		}

		if len(elements) > 0 {
			if err := tx.CreateInBatches(elements, len(elements)).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateElement updates an existing element in the database, scoped by projectID and pageID.
func (r *elementRepository) UpdateElement(ctx context.Context, projectID string, pageID string, element *domainEntity.Element) error {
	if element == nil {
		return fmt.Errorf("element is nil")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyPageOwnership(tx, projectID, pageID); err != nil {
			return err
		}
		element.PageID = pageID
		result := tx.Model(&domainEntity.Element{}).
			Select(elementMutableFields).
			Where(`"Id" = ? AND "PageId" = ?`, element.ID, pageID).
			Updates(element)

		if result.Error != nil {
			return fmt.Errorf("failed to update element: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("element not found in specified page")
		}
		return nil
	})
}

// UpdateElements updates multiple elements within a transaction, scoped by projectID and pageID.
func (r *elementRepository) UpdateElements(ctx context.Context, projectID string, pageID string, elements []domainEntity.Element) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyPageOwnership(tx, projectID, pageID); err != nil {
			return err
		}

		for i := range elements {
			elements[i].PageID = pageID
			result := tx.Model(&domainEntity.Element{}).
				Select(elementMutableFields).
				Where(`"Id" = ? AND "PageId" = ?`, elements[i].ID, pageID).
				Updates(&elements[i])

			if result.Error != nil {
				return fmt.Errorf("failed to update element %s: %w", elements[i].ID, result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("element %s not found in specified page", elements[i].ID)
			}
		}

		return nil
	})
}

// UpsertElements creates or updates multiple elements within a transaction, scoped by projectID and pageID.
func (r *elementRepository) UpsertElements(ctx context.Context, projectID string, pageID string, elements []domainEntity.Element) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyPageOwnership(tx, projectID, pageID); err != nil {
			return err
		}

		if len(elements) == 0 {
			return nil
		}

		ids := make([]string, len(elements))
		for i := range elements {
			elements[i].PageID = pageID
			ids[i] = elements[i].ID
		}

		var foreignCount int64
		if err := tx.Model(&domainEntity.Element{}).
			Where(`"Id" IN ? AND "PageId" <> ?`, ids, pageID).
			Count(&foreignCount).Error; err != nil {
			return fmt.Errorf("failed to verify element ownership: %w", err)
		}
		if foreignCount > 0 {
			return fmt.Errorf("cannot upsert element belonging to another page")
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "Id"}},
			DoUpdates: clause.AssignmentColumns([]string{"Name", "Type", "Content", "Href", "Src", "Styles", "Settings", "TailwindStyles", "Order", "OrderKey", "IsLocked", "ParentId"}),
		}).CreateInBatches(elements, len(elements)).Error; err != nil {
			return fmt.Errorf("failed to upsert elements: %w", err)
		}

		return nil
	})
}

// DeleteElement removes an element by ID, scoped by projectID and pageID.
func (r *elementRepository) DeleteElement(ctx context.Context, projectID string, pageID string, elementID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyPageOwnership(tx, projectID, pageID); err != nil {
			return err
		}

		result := tx.Where(`"Id" = ? AND "PageId" = ?`, elementID, pageID).
			Delete(&domainEntity.Element{})

		if result.Error != nil {
			return fmt.Errorf("failed to delete element: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("element not found in specified page")
		}
		return nil
	})
}

// GetElementByID retrieves a single element with its relations, scoped by projectID and pageID.
func (r *elementRepository) GetElementByID(ctx context.Context, projectID string, pageID string, elementID string) (*domainEntity.Element, error) {
	var elem domainEntity.Element
	err := r.db.WithContext(ctx).
		Model(&domainEntity.Element{}).
		Joins(`JOIN "Page" ON "Element"."PageId" = "Page"."Id"`).
		Preload("Comments").
		Preload("Comments.Author").
		Preload("EventWorkflows").
		Preload("EventWorkflows.Workflow").
		Where(`"Page"."ProjectId" = ? AND "Page"."DeletedAt" IS NULL AND "Element"."Id" = ? AND "Element"."PageId" = ?`, projectID, elementID, pageID).
		First(&elem).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get element by id: %w", err)
	}
	return &elem, nil
}

// ListElementsByPage retrieves all elements for a specific page, ordered by Order
func (r *elementRepository) ListElementsByPage(ctx context.Context, projectID string, pageID string) ([]domainEntity.Element, error) {
	var elems []domainEntity.Element
	err := r.db.WithContext(ctx).
		Model(&domainEntity.Element{}).
		Joins(`JOIN "Page" ON "Element"."PageId" = "Page"."Id"`).
		Where(`"Page"."ProjectId" = ? AND "Element"."PageId" = ?`, projectID, pageID).
		Order(`COALESCE("Element"."ParentId",'') ASC, "Element"."Order" ASC`).
		Preload("Comments").
		Preload("Comments.Author").
		Preload("EventWorkflows").
		Preload("EventWorkflows.Workflow").
		Find(&elems).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get elements by page id: %w", err)
	}
	return elems, nil
}

// ListElementsByProject retrieves all elements for a specific project
func (r *elementRepository) ListElementsByProject(ctx context.Context, projectID string) ([]domainEntity.Element, error) {
	var elems []domainEntity.Element

	err := r.db.WithContext(ctx).
		Model(&domainEntity.Element{}).
		Joins(`JOIN "Page" ON "Element"."PageId" = "Page"."Id"`).
		Where(`"Page"."ProjectId" = ?`, projectID).
		Order(`"Page"."Id" ASC, COALESCE("Element"."ParentId",'') ASC, "Element"."Order" ASC`).
		Preload("Comments").
		Preload("Comments.Author").
		Preload("EventWorkflows").
		Preload("EventWorkflows.Workflow").
		Find(&elems).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get elements by project id: %w", err)
	}
	return elems, nil
}
