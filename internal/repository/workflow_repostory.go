package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	domainEntity "webbuilder/internal/domain/entity"
	domainRepo "webbuilder/internal/domain/repository"
)

type workflowRepository struct {
	db *gorm.DB
}

// NewWorkflowRepository creates a new instance of WorkflowRepository
func NewWorkflowRepository(db *gorm.DB) domainRepo.WorkflowRepository {
	return &workflowRepository{
		db: db,
	}
}

func (r *workflowRepository) SaveWorkflow(ctx context.Context, projectID string, workflow *domainEntity.EventWorkflow) error {
	workflow.ProjectID = projectID
	if err := r.db.WithContext(ctx).Save(workflow).Error; err != nil {
		return fmt.Errorf("failed to save workflow: %w", err)
	}
	return nil
}

func (r *workflowRepository) GetWorkflow(ctx context.Context, projectID string) (*domainEntity.EventWorkflow, error) {
	var workflow domainEntity.EventWorkflow
	err := r.db.WithContext(ctx).
		Where(`"ProjectId" = ?`, projectID).
		First(&workflow).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	return &workflow, nil
}

func (r *workflowRepository) DeleteWorkflow(ctx context.Context, projectID string) error {
	if err := r.db.WithContext(ctx).
		Where(`"ProjectId" = ?`, projectID).
		Delete(&domainEntity.EventWorkflow{}).Error; err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}
	return nil
}
