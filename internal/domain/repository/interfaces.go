package repository

import (
	"context"

	"webbuilder/internal/domain/entity"
)

type ElementRepository interface {
	CreateElement(ctx context.Context, projectID string, element *entity.Element) error
	UpdateElement(ctx context.Context, projectID string, element *entity.Element) error
	UpdateElements(ctx context.Context, projectID string, elements []entity.Element) error
	DeleteElement(ctx context.Context, projectID string, elementID string) error
	GetElementByID(ctx context.Context, projectID string, elementID string) (*entity.Element, error)
	GetElementsByPageID(ctx context.Context, projectID string, pageID string) ([]entity.Element, error)
	GetElementsByProjectID(ctx context.Context, projectID string) ([]entity.Element, error)
}

type SnapshotRepository interface {
	SaveSnapshot(ctx context.Context, pageID string, name string, snapshotType entity.SnapshotType, elements []entity.Element) error
	GetSnapshot(ctx context.Context, pageID string, snapshotType entity.SnapshotType) (*entity.Snapshot, error)
	DeleteSnapshot(ctx context.Context, pageID string, snapshotType entity.SnapshotType) error
}

type DocumentRepository interface {
	SetDocument(ctx context.Context, projectID string, pageID string, elements []entity.Element) error
	GetDocument(ctx context.Context, projectID string, pageID string) ([]entity.Element, error)
	DeleteDocument(ctx context.Context, projectID string, pageID string) error
}

type WorkflowRepository interface {
	SaveWorkflow(ctx context.Context, projectID string, workflow *entity.EventWorkflow) error
	GetWorkflow(ctx context.Context, projectID string) (*entity.EventWorkflow, error)
	DeleteWorkflow(ctx context.Context, projectID string) error
}

type ProjectRepository interface {
	CreateProject(project *entity.Project) error
	GetProjectByID(projectID string) (*entity.Project, error)
	UpdateProject(project *entity.Project) error
	DeleteProject(projectID string) error
}

type ElementWorkflowRepository interface {
	CreateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error
	GetElementWorkflow(ctx context.Context, projectID string, elementID string) (*entity.ElementEventWorkflow, error)
	GetElementWorkflows(ctx context.Context, projectID string) ([]entity.ElementEventWorkflow, error)
	UpdateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error
	DeleteElementWorkflow(ctx context.Context, projectID string, elementID string) error
}

type Repository interface {
	ElementRepository
	SnapshotRepository
	DocumentRepository
	ProjectRepository
	WorkflowRepository
	ElementWorkflowRepository
}
