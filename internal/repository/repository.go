package repository

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	domainRepo "webbuilder/internal/domain/repository"
)

// Repository is a composition of all repositories that implements domainRepo.Repository.
// By embedding the individual repository interfaces, it satisfies the composite Repository interface
// defined in the domain layer.
type Repository struct {
	domainRepo.ElementRepository
	domainRepo.SnapshotRepository
	domainRepo.DocumentRepository
	domainRepo.ProjectRepository
	domainRepo.WorkflowRepository
	domainRepo.ElementWorkflowRepository
}

// NewRepository creates a new instance of Repository that satisfies the domain Repository interface.
// It requires both a GORM DB connection for persistent storage and a Redis client for caching/real-time document state.
func NewRepository(db *gorm.DB, rdb *redis.Client) domainRepo.Repository {
	return &Repository{
		ElementRepository:         NewElementRepository(db),
		SnapshotRepository:        NewSnapshotRepository(db),
		DocumentRepository:        NewDocumentRepository(rdb),
		ProjectRepository:         NewProjectRepository(db),
		WorkflowRepository:        NewWorkflowRepository(db),
		ElementWorkflowRepository: NewElementWorkflowRepository(db),
	}
}
