package repository

import (
	"context"

	"webbuilder/internal/domain/entity"
)

type ElementRepository interface {
	CreateElement(ctx context.Context, projectID string, pageID string, element *entity.Element) error
	CreateElements(ctx context.Context, projectID string, pageID string, element []entity.Element) error
	UpdateElement(ctx context.Context, projectID string, pageID string, element *entity.Element) error
	UpdateElements(ctx context.Context, projectID string, pageID string, elements []entity.Element) error
	DeleteElement(ctx context.Context, projectID string, pageID string, elementID string) error
	GetElementByID(ctx context.Context, projectID string, pageID string, elementID string) (*entity.Element, error)
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

	MarkPageDirty(ctx context.Context, projectID string, pageId string) error
	PopDirtyPages(ctx context.Context, limit int) ([]entity.DirtyPage, error)
}

type WorkflowRepository interface {
	CreateWorkflow(ctx context.Context, projectID string, workflow *entity.EventWorkflow) error
	UpdateWorkflow(ctx context.Context, projectID string, workflow *entity.EventWorkflow) error
	DeleteWorkflow(ctx context.Context, projectID string, workflowID string) error
	GetWorkflowByID(ctx context.Context, projectID string, workflowID string) (*entity.EventWorkflow, error)
	GetWorkflowsByProjectID(ctx context.Context, projectID string) ([]entity.EventWorkflow, error)
}

type ProjectRepository interface {
	CreateProject(project *entity.Project) error
	GetProjectByID(projectID string) (*entity.Project, error)
	UpdateProject(project *entity.Project) error
	DeleteProject(projectID string) error
}

type ElementWorkflowRepository interface {
	CreateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error
	UpdateElementWorkflow(ctx context.Context, projectID string, workflow *entity.ElementEventWorkflow) error
	DeleteElementWorkflow(ctx context.Context, projectID string, elementID string) error
	GetElementWorkflowByElementID(ctx context.Context, projectID string, elementID string) (*entity.ElementEventWorkflow, error)
	GetElementWorkflowsByProjectID(ctx context.Context, projectID string) ([]entity.ElementEventWorkflow, error)
	GetElementWorkflowsByPageID(ctx context.Context, projectID string, pageID string) ([]entity.ElementEventWorkflow, error)
}

type Repository struct {
	ElementRepo         ElementRepository
	SnapshotRepo        SnapshotRepository
	DocumentRepo        DocumentRepository
	ProjectRepo         ProjectRepository
	WorkflowRepo        WorkflowRepository
	ElementWorkflowRepo ElementWorkflowRepository
}
