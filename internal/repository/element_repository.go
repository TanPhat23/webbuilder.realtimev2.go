package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	domainEntity "webbuilder/internal/domain/entity"
	domainRepo "webbuilder/internal/domain/repository"
)

type elementRepository struct {
	db *gorm.DB
}

// NewElementRepository creates a new instance of ElementRepository
func NewElementRepository(db *gorm.DB) domainRepo.ElementRepository {
	return &elementRepository{db: db}
}

// CreateElement persists a new element to the database
func (r *elementRepository) CreateElement(ctx context.Context, projectID string, element *domainEntity.Element) error {
	if err := r.db.WithContext(ctx).Create(element).Error; err != nil {
		return fmt.Errorf("failed to create element: %w", err)
	}
	return nil
}

// UpdateElement updates an existing element in the database
func (r *elementRepository) UpdateElement(ctx context.Context, projectID string, element *domainEntity.Element) error {
	if err := r.db.WithContext(ctx).Save(element).Error; err != nil {
		return fmt.Errorf("failed to update element: %w", err)
	}
	return nil
}

// UpdateElements updates multiple elements within a transaction
func (r *elementRepository) UpdateElements(ctx context.Context, projectID string, elements []domainEntity.Element) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	for i := range elements {
		if err := tx.Save(&elements[i]).Error; err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update element: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit updates: %w", err)
	}
	return nil
}

// DeleteElement removes an element by ID
func (r *elementRepository) DeleteElement(ctx context.Context, projectID string, elementID string) error {
	if err := r.db.WithContext(ctx).Delete(&domainEntity.Element{}, `"Id" = ?`, elementID).Error; err != nil {
		return fmt.Errorf("failed to delete element: %w", err)
	}
	return nil
}

// GetElementByID retrieves a single element with its relations
func (r *elementRepository) GetElementByID(ctx context.Context, projectID string, elementID string) (*domainEntity.Element, error) {
	var elem domainEntity.Element
	err := r.db.WithContext(ctx).
		Preload("Setting").
		Preload("Comments").
		Preload("Comments.Author").
		Preload("EventWorkflows").
		Preload("EventWorkflows.Workflow").
		First(&elem, `"Id" = ?`, elementID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get element by id: %w", err)
	}
	return &elem, nil
}

// GetElementsByPageID retrieves all elements for a specific page, ordered by Order
func (r *elementRepository) GetElementsByPageID(ctx context.Context, projectID string, pageID string) ([]domainEntity.Element, error) {
	var elems []domainEntity.Element

	err := r.db.WithContext(ctx).
		Model(&domainEntity.Element{}).
		Joins(`JOIN "Page" ON "Element"."PageId" = "Page"."Id"`).
		Where(`"Page"."ProjectId" = ? AND "Element"."PageId" = ?`, projectID, pageID).
		Order(`"Element"."Order" ASC`).
		Preload("Setting").
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

// GetElementsByProjectID retrieves all elements for a specific project
func (r *elementRepository) GetElementsByProjectID(ctx context.Context, projectID string) ([]domainEntity.Element, error) {
	var elems []domainEntity.Element

	err := r.db.WithContext(ctx).
		Model(&domainEntity.Element{}).
		Joins(`JOIN "Page" ON "Element"."PageId" = "Page"."Id"`).
		Where(`"Page"."ProjectId" = ?`, projectID).
		Order(`"Page"."Id" ASC, "Element"."Order" ASC`).
		Preload("Setting").
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
