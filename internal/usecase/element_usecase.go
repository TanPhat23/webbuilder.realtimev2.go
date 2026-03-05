package usecase

import (
	"context"
	"fmt"

	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	domainRepo "webbuilder/internal/domain/repository"
	domainUsecase "webbuilder/internal/domain/usecase"
	elementutils "webbuilder/internal/utils/element"
	"webbuilder/internal/utils/validation"

	"github.com/google/uuid"
)

type elementUseCase struct {
	repo           *domainRepo.Repository
	docUseCase     domainUsecase.DocumentUseCase
	persistUseCase domainUsecase.PersistUseCase
}

// NewElementUseCase creates a new instance of elementUseCase with persistence integration.
func NewElementUseCase(
	repo *domainRepo.Repository,
	docUseCase domainUsecase.DocumentUseCase,
	persistUseCase domainUsecase.PersistUseCase,
) domainUsecase.ElementUseCase {
	return &elementUseCase{
		repo:           repo,
		docUseCase:     docUseCase,
		persistUseCase: persistUseCase,
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
	return uc.docUseCase.GetDocument(ctx, projectID, pageID)
}

func (uc *elementUseCase) GetElementsByProjectID(ctx context.Context, projectID string) ([]domainEntity.Element, error) {
	flattenElements, err := uc.repo.ElementRepo.GetElementsByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return elementutils.BuildElementTree(flattenElements), nil
}

func (uc *elementUseCase) CreateElement(ctx context.Context, projectID string, element *domainEntity.Element) error {
	if err := validation.ValidateElementForAdd(element); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if element.ID == "" {
		element.ID = uuid.NewString()
	}

	elements, err := uc.docUseCase.GetDocument(ctx, projectID, element.PageID)
	if err != nil {
		return err
	}

	if element.ParentID != nil && *element.ParentID != "" {
		inserted := false
		for i := range elements {
			if elementutils.CreateElement(&elements[i], *element) {
				inserted = true
				break
			}
		}
		if !inserted {
			if *element.ParentID == "root" {
				root := domainEntity.Element{
					ID:       "root",
					Elements: []domainEntity.Element{*element},
				}
				elements = append(elements, root)
			} else {
				return domainErrors.New(domainErrors.CodeNotFound, "parent not found", nil)
			}
		}
	} else {
		element.Order = len(elements)
		elements = append(elements, *element)
	}

	// Write-Back Strategy: Update Cache, then Mark Dirty
	if err := uc.docUseCase.AddElementsToDocument(ctx, projectID, element.PageID, elements); err != nil {
		return err
	}

	return uc.persistUseCase.MarkDirty(ctx, projectID, element.PageID)
}

func (uc *elementUseCase) UpdateElement(ctx context.Context, projectID string, pageID string, element *domainEntity.Element) error {
	if element == nil || element.ID == "" {
		return domainErrors.New(domainErrors.CodeValidation, "element id is required", nil)
	}

	elements, err := uc.docUseCase.GetDocument(ctx, projectID, pageID)
	if err != nil {
		return err
	}

	found := false
	for i := range elements {
		if elementutils.UpdateElement(&elements[i], *element) {
			found = true
			break
		}
	}

	if !found {
		return domainErrors.New(domainErrors.CodeNotFound, "element not found in cache", nil)
	}

	if err := uc.docUseCase.AddElementsToDocument(ctx, projectID, pageID, elements); err != nil {
		return err
	}

	return uc.persistUseCase.MarkDirty(ctx, projectID, pageID)
}

func (uc *elementUseCase) PatchElement(ctx context.Context, projectID string, pageID string, elementID string, properties map[string]any, styles map[string]any, settings map[string]any) error {
	elements, err := uc.docUseCase.GetDocument(ctx, projectID, pageID)
	if err != nil {
		return err
	}

	flat := elementutils.FlattenElements(elements)
	found := false
	for i := range flat {
		if flat[i].ID == elementID {
			if properties != nil {
				flat[i].Settings = properties
			}
			if styles != nil {
				flat[i].Styles = styles
			}
			if settings != nil {
				flat[i].Settings = settings
			}
			found = true
			break
		}
	}

	if !found {
		return domainErrors.New(domainErrors.CodeNotFound, "element not found in cache", nil)
	}

	newTree := elementutils.BuildElementTree(flat)
	if err := uc.docUseCase.AddElementsToDocument(ctx, projectID, pageID, newTree); err != nil {
		return err
	}

	return uc.persistUseCase.MarkDirty(ctx, projectID, pageID)
}

func (uc *elementUseCase) MoveElement(ctx context.Context, projectID string, pageID string, elementID string, parentID string, index int) (*domainEntity.Element, error) {
	elements, err := uc.docUseCase.GetDocument(ctx, projectID, pageID)
	if err != nil {
		return nil, err
	}

	flat := elementutils.FlattenElements(elements)
	var target *domainEntity.Element
	for i := range flat {
		if flat[i].ID == elementID {
			target = &flat[i]
			break
		}
	}

	if target == nil {
		return nil, domainErrors.New(domainErrors.CodeNotFound, "element not found", nil)
	}

	updatedFlat := uc.calculateMoveReordering(target, flat, parentID, index)

	newTree := elementutils.BuildElementTree(updatedFlat)
	if err := uc.docUseCase.AddElementsToDocument(ctx, projectID, pageID, newTree); err != nil {
		return nil, err
	}

	if err := uc.persistUseCase.MarkDirty(ctx, projectID, pageID); err != nil {
		return nil, err
	}

	for _, e := range updatedFlat {
		if e.ID == elementID {
			return &e, nil
		}
	}

	return target, nil
}

func (uc *elementUseCase) RemoveElement(ctx context.Context, projectID string, pageID string, elementID string) error {
	elements, err := uc.docUseCase.GetDocument(ctx, projectID, pageID)
	if err != nil {
		return err
	}

	removed := false
	newRoots := make([]domainEntity.Element, 0, len(elements))
	for _, r := range elements {
		if r.ID == elementID {
			removed = true
			continue
		}
		newRoots = append(newRoots, r)
	}
	if removed {
		elements = newRoots
	} else {
		for i := range elements {
			if elementutils.DeleteElement(&elements[i], elementID) {
				removed = true
				break
			}
		}
	}

	if err := uc.docUseCase.AddElementsToDocument(ctx, projectID, pageID, elements); err != nil {
		return err
	}

	return uc.persistUseCase.MarkDirty(ctx, projectID, pageID)
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
	updatedTarget = append(updatedTarget, moved)
	updatedTarget = append(updatedTarget, newSiblings[targetIndex:]...)

	// Assign orders for target siblings
	for i := range updatedTarget {
		updatedTarget[i].Order = i
	}

	// Assign orders for old siblings if they changed
	var toUpdate []domainEntity.Element
	toUpdate = append(toUpdate, updatedTarget...)

	if !elementutils.CompareParent(origParent, targetParentID) {
		for i := range oldSiblings {
			oldSiblings[i].Order = i
			toUpdate = append(toUpdate, oldSiblings[i])
		}
	}

	// Merge back with non-involved elements
	resultMap := make(map[string]domainEntity.Element)
	for _, e := range all {
		resultMap[e.ID] = e
	}
	for _, e := range toUpdate {
		resultMap[e.ID] = e
	}

	final := make([]domainEntity.Element, 0, len(all))
	for _, e := range resultMap {
		final = append(final, e)
	}

	return final
}
