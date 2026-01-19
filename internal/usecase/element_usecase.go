package usecase

import (
	"context"
	"fmt"
	"time"

	elementutils "webbuilder/internal/utils/element"

	"github.com/google/uuid"

	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	domainRepo "webbuilder/internal/domain/repository"
	domainUsecase "webbuilder/internal/domain/usecase"
	"webbuilder/internal/utils/validation"
)

type elementUseCase struct {
	elementRepo domainRepo.ElementRepository
	docUseCase  domainUsecase.DocumentUseCase
}

// NewElementUseCase creates a new instance of elementUseCase
func NewElementUseCase(elementRepo domainRepo.ElementRepository, docUseCase domainUsecase.DocumentUseCase) domainUsecase.ElementUseCase {
	return &elementUseCase{
		elementRepo: elementRepo,
		docUseCase:  docUseCase,
	}
}

// SyncPage ensures the cache is up to date for a specific page.
func (uc *elementUseCase) SyncPage(ctx context.Context, projectID string, pageID string) ([]domainEntity.Element, error) {
	if err := uc.docUseCase.RefreshCache(ctx, projectID, pageID); err != nil {
		return nil, fmt.Errorf("failed to refresh cache: %w", err)
	}
	return uc.docUseCase.GetDocument(ctx, projectID, pageID)
}



func (uc *elementUseCase) GetElementsByPageID(ctx context.Context, projectID string, pageID string) ([]domainEntity.Element, error) {
	return uc.elementRepo.GetElementsByPageID(ctx, projectID, pageID)
}

func (uc *elementUseCase) GetElementsByProjectID(ctx context.Context, projectID string) ([]domainEntity.Element, error) {
	return uc.elementRepo.GetElementsByProjectID(ctx, projectID)
}

func (uc *elementUseCase) AddElement(ctx context.Context, projectID string, element *domainEntity.Element) error {
	if err := validation.ValidateElementForAdd(element); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if element.ID == "" {
		element.ID = uuid.NewString()
	}

	uc.setTimestamps(element, true)

	if err := uc.elementRepo.CreateElement(ctx, projectID, element); err != nil {
		return fmt.Errorf("failed to create element in db: %w", err)
	}

	return uc.docUseCase.RefreshCache(ctx, projectID, element.PageID)
}

func (uc *elementUseCase) UpdateElement(ctx context.Context, projectID string, element *domainEntity.Element) error {
	if element == nil || element.ID == "" {
		return domainErrors.New(domainErrors.CodeValidation, "element id is required", nil)
	}

	if err := validation.ValidateElementForUpdate(element); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	existing, err := uc.getExistingElement(ctx, projectID, element.ID)
	if err != nil {
		return err
	}

	if element.PageID == "" {
		element.PageID = existing.PageID
	}

	uc.setTimestamps(element, false)

	if err := uc.elementRepo.UpdateElement(ctx, projectID, element); err != nil {
		return fmt.Errorf("failed to update element in db: %w", err)
	}

	return uc.docUseCase.RefreshCache(ctx, projectID, element.PageID)
}

func (uc *elementUseCase) PatchElement(ctx context.Context, projectID string, elementID string, properties map[string]any, styles map[string]any, settings *domainEntity.Setting) error {
	existing, err := uc.getExistingElement(ctx, projectID, elementID)
	if err != nil {
		return err
	}

	if properties != nil {
		existing.Properties = properties
	}
	if styles != nil {
		existing.Styles = styles
	}
	if settings != nil {
		existing.Setting = settings
	}

	uc.setTimestamps(existing, false)

	if err := uc.elementRepo.UpdateElement(ctx, projectID, existing); err != nil {
		return fmt.Errorf("failed to update element in db: %w", err)
	}

	return uc.docUseCase.RefreshCache(ctx, projectID, existing.PageID)
}

func (uc *elementUseCase) MoveElement(ctx context.Context, projectID string, elementID string, parentID string, index int) (*domainEntity.Element, error) {
	element, err := uc.getExistingElement(ctx, projectID, elementID)
	if err != nil {
		return nil, err
	}

	if err := uc.validateMoveTarget(ctx, projectID, element, parentID, index); err != nil {
		return nil, err
	}

	all, err := uc.elementRepo.GetElementsByPageID(ctx, projectID, element.PageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch elements for page: %w", err)
	}

	toUpdate := uc.calculateMoveReordering(element, all, parentID, index)

	if err := uc.elementRepo.UpdateElements(ctx, projectID, toUpdate); err != nil {
		return nil, fmt.Errorf("failed to update elements order in db: %w", err)
	}

	// Fetch fresh copy after move
	updatedElem, err := uc.elementRepo.GetElementByID(ctx, projectID, element.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated element: %w", err)
	}

	if err := uc.docUseCase.RefreshCache(ctx, projectID, element.PageID); err != nil {
		return updatedElem, fmt.Errorf("failed to refresh cache after move: %w", err)
	}

	return updatedElem, nil
}

func (uc *elementUseCase) RemoveElement(ctx context.Context, projectID string, elementID string) error {
	element, err := uc.elementRepo.GetElementByID(ctx, projectID, elementID)
	if err != nil {
		return fmt.Errorf("failed to fetch element for deletion: %w", err)
	}
	if element == nil {
		return nil
	}

	if err := uc.elementRepo.DeleteElement(ctx, projectID, elementID); err != nil {
		return fmt.Errorf("failed to delete element from db: %w", err)
	}

	return uc.docUseCase.RefreshCache(ctx, projectID, element.PageID)
}

// Internal Helpers

func (uc *elementUseCase) getExistingElement(ctx context.Context, projectID, elementID string) (*domainEntity.Element, error) {
	if elementID == "" {
		return nil, domainErrors.New(domainErrors.CodeValidation, "element id is required", nil)
	}
	existing, err := uc.elementRepo.GetElementByID(ctx, projectID, elementID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing element: %w", err)
	}
	if existing == nil {
		return nil, domainErrors.New(domainErrors.CodeNotFound, "element not found", nil)
	}
	return existing, nil
}

func (uc *elementUseCase) setTimestamps(element *domainEntity.Element, isNew bool) {
	now := time.Now()
	if isNew {
		element.CreatedAt = now
	}
	element.UpdatedAt = now
}

func (uc *elementUseCase) validateMoveTarget(ctx context.Context, projectID string, element *domainEntity.Element, parentID string, index int) error {
	if index < 0 {
		return domainErrors.New(domainErrors.CodeValidation, "index must be non-negative", nil)
	}
	if parentID != "" {
		parent, err := uc.elementRepo.GetElementByID(ctx, projectID, parentID)
		if err != nil || parent == nil {
			return domainErrors.New(domainErrors.CodeNotFound, "parent element not found", nil)
		}
		if parent.PageID != element.PageID {
			return domainErrors.New(domainErrors.CodeValidation, "parent must belong to same page", nil)
		}
	}
	return nil
}

func (uc *elementUseCase) calculateMoveReordering(element *domainEntity.Element, all []domainEntity.Element, targetParentID string, targetIndex int) []domainEntity.Element {
	origParent := element.ParentID
	var oldSiblings []domainEntity.Element
	var newSiblings []domainEntity.Element

	// Split into sibling sets
	for _, e := range all {
		if elementutils.ParentsEqual(e.ParentID, origParent) && e.ID != element.ID {
			oldSiblings = append(oldSiblings, e)
		}
		if elementutils.CompareParent(e.ParentID, targetParentID) && e.ID != element.ID {
			newSiblings = append(newSiblings, e)
		}
	}

	// Handle move within the same parent
	if elementutils.CompareParent(origParent, targetParentID) {
		newSiblings = oldSiblings
	}

	// Clamp index
	if targetIndex > len(newSiblings) {
		targetIndex = len(newSiblings)
	}

	// Build updated target list
	updatedTarget := make([]domainEntity.Element, 0, len(newSiblings)+1)
	updatedTarget = append(updatedTarget, newSiblings[:targetIndex]...)

	moved := *element
	if targetParentID == "" {
		moved.ParentID = nil
	} else {
		pID := targetParentID
		moved.ParentID = &pID
	}
	uc.setTimestamps(&moved, false)
	updatedTarget = append(updatedTarget, moved)
	updatedTarget = append(updatedTarget, newSiblings[targetIndex:]...)

	// Assign orders for target siblings
	for i := range updatedTarget {
		updatedTarget[i].Order = i
		uc.setTimestamps(&updatedTarget[i], false)
	}

	// Assign orders for old siblings if they changed
	var toUpdate []domainEntity.Element
	toUpdate = append(toUpdate, updatedTarget...)

	if !elementutils.CompareParent(origParent, targetParentID) {
		for i := range oldSiblings {
			oldSiblings[i].Order = i
			uc.setTimestamps(&oldSiblings[i], false)
			toUpdate = append(toUpdate, oldSiblings[i])
		}
	}

	return toUpdate
}
