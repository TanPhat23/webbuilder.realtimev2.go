package usecase

import (
	"context"
	"fmt"

	domainEntity "webbuilder/internal/domain/entity"
	domainRepo "webbuilder/internal/domain/repository"
	domainUsecase "webbuilder/internal/domain/usecase"
)

type documentUseCase struct {
	repo *domainRepo.Repository
}

// NewDocumentUseCase creates a new instance of documentUseCase
func NewDocumentUseCase(repo *domainRepo.Repository) domainUsecase.DocumentUseCase {
	return &documentUseCase{
		repo: repo,
	}
}

func (uc *documentUseCase) GetDocument(ctx context.Context, projectID string, pageID string) ([]domainEntity.Element, error) {
	cachedElements, err := uc.repo.DocumentRepo.GetDocument(ctx, projectID, pageID)
	if err != nil {
		fmt.Printf("cache error during get: %v\n", err)
	}
	if cachedElements != nil {
		return cachedElements, nil
	}

	elements, err := uc.repo.ElementRepo.GetElementsByPageID(ctx, projectID, pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page elements from db: %w", err)
	}

	if err := uc.repo.DocumentRepo.SetDocument(ctx, projectID, pageID, elements); err != nil {
		fmt.Printf("failed to update cache: %v\n", err)
	}

	return elements, nil
}

func (uc *documentUseCase) RefreshCache(ctx context.Context, projectID string, pageID string) error {
	elements, err := uc.repo.ElementRepo.GetElementsByPageID(ctx, projectID, pageID)
	if err != nil {
		return fmt.Errorf("failed to fetch elements for cache refresh: %w", err)
	}

	if err := uc.repo.DocumentRepo.SetDocument(ctx, projectID, pageID, elements); err != nil {
		return fmt.Errorf("failed to refresh cache: %w", err)
	}

	return nil
}

func (uc *documentUseCase) AddElementsToDocument(ctx context.Context, projectID string, pageID string, elements []domainEntity.Element) error {
	if err := uc.repo.DocumentRepo.SetDocument(ctx, projectID, pageID, elements); err != nil {
		return fmt.Errorf("failed to set document elements in cache: %w", err)
	}
	return nil
}
