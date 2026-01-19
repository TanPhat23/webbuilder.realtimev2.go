package usecase

import (
	domainRepo "webbuilder/internal/domain/repository"
	domainUsecase "webbuilder/internal/domain/usecase"
	"webbuilder/internal/utils/logger"
)

// NewUseCase initializes all use cases and returns a central container.
func NewUseCase(repo *domainRepo.Repository, lg logger.Logger) *domainUsecase.UseCase {
	docUC := NewDocumentUseCase(repo)
	elemUC := NewElementUseCase(repo, docUC)
	persistUC := NewPersistenceUsecase(repo, lg)

	return &domainUsecase.UseCase{
		Element:  elemUC,
		Document: docUC,
		Persist:  persistUC,
	}
}
