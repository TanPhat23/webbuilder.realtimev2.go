package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"webbuilder/internal/domain/entity"
	domainRepo "webbuilder/internal/domain/repository"
)

type elementWorkflowRepository struct {
	db *gorm.DB
}

// NewElementWorkflowRepository creates a new instance of ElementWorkflowRepository
func NewElementWorkflowRepository(db *gorm.DB) domainRepo.ElementWorkflowRepository {
	return &elementWorkflowRepository{db: db}
}

// projectScope leverages GORM's relation-based joins to filter records by ProjectId.
func (r *elementWorkflowRepository) projectScope(projectID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Joins("Element").
			Joins("Element.Page").
			Where("\"Element__Page\".\"ProjectId\" = ?", projectID)
	}
}

// pageScope leverages GORM's relation-based joins to filter records by ProjectId and PageId.
func (r *elementWorkflowRepository) pageScope(projectID string, pageID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Joins("Element").
			Joins("Element.Page").
			Where("\"Element__Page\".\"ProjectId\" = ? AND \"Element\".\"PageId\" = ?", projectID, pageID)
	}
}

// elementsInProject is a helper subquery to find all element IDs belonging to a specific project.
func (r *elementWorkflowRepository) elementsInProject(projectID string) *gorm.DB {
	return r.elementsInProjectTx(r.db, projectID)
}

func (r *elementWorkflowRepository) elementsInProjectTx(db *gorm.DB, projectID string) *gorm.DB {
	return db.Model(&entity.Element{}).
		Select("\"Element\".\"Id\"").
		Joins("Page").
		Where("\"Page\".\"ProjectId\" = ? AND \"Page\".\"DeletedAt\" IS NULL", projectID)
}

func verifyElementWorkflowOwnership(tx *gorm.DB, projectID, elementID, workflowID string) error {
	var count int64
	err := tx.Model(&entity.Element{}).
		Joins("Page").
		Where("\"Element\".\"Id\" = ? AND \"Page\".\"ProjectId\" = ? AND \"Page\".\"DeletedAt\" IS NULL", elementID, projectID).
		Count(&count).Error
	if err != nil || count == 0 {
		return fmt.Errorf("invalid element for project: %w", err)
	}

	err = tx.Model(&entity.EventWorkflow{}).
		Where("\"Id\" = ? AND \"ProjectId\" = ?", workflowID, projectID).
		Count(&count).Error
	if err != nil || count == 0 {
		return fmt.Errorf("invalid workflow for project: %w", err)
	}
	return nil
}

func (r *elementWorkflowRepository) CreateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error {
	if workflow == nil {
		return errors.New("element workflow is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyElementWorkflowOwnership(tx, projectID, workflow.ElementID, workflow.WorkflowID); err != nil {
			return err
		}
		if err := tx.Create(workflow).Error; err != nil {
			return fmt.Errorf("failed to create element workflow: %w", err)
		}
		return nil
	})
}

func (r *elementWorkflowRepository) GetElementWorkflowByElementID(ctx context.Context, projectID string, elementID string) (*entity.ElementEventWorkflow, error) {
	var workflow entity.ElementEventWorkflow
	err := r.db.WithContext(ctx).
		Preload("Element").
		Preload("Workflow").
		Scopes(r.projectScope(projectID)).
		Where("\"ElementEventWorkflow\".\"ElementId\" = ?", elementID).
		First(&workflow).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get element workflow: %w", err)
	}
	return &workflow, nil
}

func (r *elementWorkflowRepository) GetElementWorkflowsByProjectID(ctx context.Context, projectID string) ([]entity.ElementEventWorkflow, error) {
	var workflows []entity.ElementEventWorkflow
	err := r.db.WithContext(ctx).
		Preload("Element").
		Preload("Workflow").
		Scopes(r.projectScope(projectID)).
		Find(&workflows).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get element workflows: %w", err)
	}
	return workflows, nil
}

func (r *elementWorkflowRepository) GetElementWorkflowsByPageID(ctx context.Context, projectID string, pageID string) ([]entity.ElementEventWorkflow, error) {
	var workflows []entity.ElementEventWorkflow
	err := r.db.WithContext(ctx).
		Preload("Element").
		Preload("Workflow").
		Scopes(r.pageScope(projectID, pageID)).
		Find(&workflows).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get element workflows by page: %w", err)
	}
	return workflows, nil
}

func (r *elementWorkflowRepository) UpdateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error {
	if workflow == nil || workflow.ID == "" {
		return errors.New("element workflow id is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyElementWorkflowOwnership(tx, projectID, workflow.ElementID, workflow.WorkflowID); err != nil {
			return err
		}

		result := tx.Model(&entity.ElementEventWorkflow{}).
			Select("ElementID", "WorkflowID", "EventName").
			Where("\"Id\" = ? AND \"ElementId\" IN (?)", workflow.ID, r.elementsInProjectTx(tx, projectID)).
			Updates(workflow)

		if result.Error != nil {
			return fmt.Errorf("failed to update element workflow: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.New("element workflow not found or unauthorized")
		}
		return nil
	})
}

func (r *elementWorkflowRepository) DeleteElementWorkflow(ctx context.Context, projectID string, elementID string) error {
	result := r.db.WithContext(ctx).
		Where("\"ElementId\" = ? AND \"ElementId\" IN (?)", elementID, r.elementsInProject(projectID)).
		Delete(&entity.ElementEventWorkflow{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete element workflow: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("element workflow not found or unauthorized")
	}
	return nil
}
