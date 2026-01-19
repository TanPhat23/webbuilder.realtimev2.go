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

// CreateElementWorkflow persists a new link between an element and a workflow
func (r *elementWorkflowRepository) CreateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error {
	if err := r.db.WithContext(ctx).Create(workflow).Error; err != nil {
		return fmt.Errorf("failed to create element workflow: %w", err)
	}
	return nil
}

// GetElementWorkflow retrieves a singular event workflow for a specific element, verified by projectID
func (r *elementWorkflowRepository) GetElementWorkflow(ctx context.Context, projectID string, elementID string) (*entity.ElementEventWorkflow, error) {
	var workflow entity.ElementEventWorkflow
	err := r.db.WithContext(ctx).
		Table("ElementEventWorkflow").
		Joins("JOIN \"Element\" ON \"ElementEventWorkflow\".\"ElementId\" = \"Element\".\"Id\"").
		Joins("JOIN \"Page\" ON \"Element\".\"PageId\" = \"Page\".\"Id\"").
		Where("\"Page\".\"ProjectId\" = ? AND \"ElementEventWorkflow\".\"ElementId\" = ?", projectID, elementID).
		First(&workflow).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get element workflow: %w", err)
	}
	return &workflow, nil
}

// GetElementWorkflows retrieves all element-workflow mappings for an entire project
func (r *elementWorkflowRepository) GetElementWorkflows(ctx context.Context, projectID string) ([]entity.ElementEventWorkflow, error) {
	var workflows []entity.ElementEventWorkflow
	err := r.db.WithContext(ctx).
		Table("ElementEventWorkflow").
		Joins("JOIN \"Element\" ON \"ElementEventWorkflow\".\"ElementId\" = \"Element\".\"Id\"").
		Joins("JOIN \"Page\" ON \"Element\".\"PageId\" = \"Page\".\"Id\"").
		Where("\"Page\".\"ProjectId\" = ?", projectID).
		Find(&workflows).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get element workflows: %w", err)
	}
	return workflows, nil
}

// UpdateElementWorkflow updates an existing element-workflow mapping
func (r *elementWorkflowRepository) UpdateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error {
	if err := r.db.WithContext(ctx).Save(workflow).Error; err != nil {
		return fmt.Errorf("failed to update element workflow: %w", err)
	}
	return nil
}

// DeleteElementWorkflow removes all workflow mappings for a specific element, verified by projectID
func (r *elementWorkflowRepository) DeleteElementWorkflow(ctx context.Context, projectID string, elementID string) error {
	// Security check: ensure the element belongs to the project before deleting its workflows
	err := r.db.WithContext(ctx).
		Where("\"ElementId\" = ? AND \"ElementId\" IN (SELECT \"Element\".\"Id\" FROM \"Element\" JOIN \"Page\" ON \"Element\".\"PageId\" = \"Page\".\"Id\" WHERE \"Page\".\"ProjectId\" = ?)", elementID, projectID).
		Delete(&entity.ElementEventWorkflow{}).Error

	if err != nil {
		return fmt.Errorf("failed to delete element workflow: %w", err)
	}
	return nil
}
