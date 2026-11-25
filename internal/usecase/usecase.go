package usecase

import (
	domainRepo "webbuilder/internal/domain/repository"
	domainUsecase "webbuilder/internal/domain/usecase"
	"webbuilder/internal/utils/logger"
)

// New initializes all use cases and returns a central container.
func New(repo *domainRepo.Repository, lg logger.Logger) *domainUsecase.UseCase {
	docUC := NewDocumentUseCase(repo)
	persistUC := NewPersistUseCase(repo, lg)
	elemUC := NewElementUseCase(repo, docUC, persistUC)
	workflowUC := NewWorkflowUseCase(repo.WorkflowRepo)
	elementWorkflowUC := NewElementWorkflowUseCase(repo)

	return &domainUsecase.UseCase{
		Logic:            NewLogicUseCase(repo.LogicRepo),
		Element:          elemUC,
		Document:         docUC,
		Persist:          persistUC,
		Workflow:         workflowUC,
		ElementWorkflows: elementWorkflowUC,
	}
}
