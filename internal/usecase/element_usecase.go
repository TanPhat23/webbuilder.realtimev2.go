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

// SyncPageWithSeq is SyncPage plus the page sequence number belonging to the returned elements.
func (uc *elementUseCase) SyncPageWithSeq(ctx context.Context, projectID string, pageID string) ([]domainEntity.Element, int64, error) {
	if err := uc.docUseCase.RefreshCache(ctx, projectID, pageID); err != nil {
		return nil, 0, fmt.Errorf("failed to refresh cache: %w", err)
	}
	return uc.docUseCase.GetDocumentWithSeq(ctx, projectID, pageID)
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

func (uc *elementUseCase) CreateElement(ctx context.Context, projectID string, element *domainEntity.Element) (int64, error) {
	if err := validation.ValidateElementForAdd(element); err != nil {
		return 0, domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if element.ID == "" {
		element.ID = uuid.NewString()
	}

	return uc.docUseCase.MutateDocument(ctx, projectID, element.PageID, func(elements []domainEntity.Element) ([]domainEntity.Element, error) {
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
					return nil, domainErrors.New(domainErrors.CodeNotFound, "parent not found", nil)
				}
			}
		} else {
			element.Order = len(elements)
			elements = append(elements, *element)
		}
		return elements, nil
	})
}

func (uc *elementUseCase) UpdateElement(ctx context.Context, projectID string, pageID string, element *domainEntity.Element) (int64, error) {
	if element == nil || element.ID == "" {
		return 0, domainErrors.New(domainErrors.CodeValidation, "element id is required", nil)
	}

	return uc.docUseCase.MutateDocument(ctx, projectID, pageID, func(elements []domainEntity.Element) ([]domainEntity.Element, error) {
		for i := range elements {
			if elementutils.UpdateElement(&elements[i], *element) {
				return elements, nil
			}
		}
		return nil, domainErrors.New(domainErrors.CodeNotFound, "element not found in cache", nil)
	})
}

// PatchElement merges the given changes into the element in place:
//   - styles and settings are deep-merged into the existing maps (a null value deletes the key);
//   - properties are applied to the element's own fields (see elementutils.ApplyProperties);
//     they are never written into Settings.
func (uc *elementUseCase) PatchElement(ctx context.Context, projectID string, pageID string, elementID string, properties map[string]any, styles map[string]any, settings map[string]any) (int64, error) {
	return uc.docUseCase.MutateDocument(ctx, projectID, pageID, func(elements []domainEntity.Element) ([]domainEntity.Element, error) {
		target := elementutils.FindByID(elements, elementID)
		if target == nil {
			return nil, domainErrors.New(domainErrors.CodeNotFound, "element not found in cache", nil)
		}

		if len(properties) > 0 {
			if err := elementutils.ApplyProperties(target, properties); err != nil {
				return nil, domainErrors.Wrap(domainErrors.CodeValidation, err)
			}
		}
		if len(styles) > 0 {
			target.Styles = elementutils.MergeMap(target.Styles, styles)
		}
		if len(settings) > 0 {
			target.Settings = elementutils.MergeMap(target.Settings, settings)
		}
		return elements, nil
	})
}

func (uc *elementUseCase) MoveElement(ctx context.Context, projectID string, pageID string, elementID string, parentID string, index int) (*domainEntity.Element, int64, error) {
	var result *domainEntity.Element

	seq, err := uc.docUseCase.MutateDocument(ctx, projectID, pageID, func(elements []domainEntity.Element) ([]domainEntity.Element, error) {
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

		result = target
		for i := range updatedFlat {
			if updatedFlat[i].ID == elementID {
				moved := updatedFlat[i]
				result = &moved
				break
			}
		}

		return elementutils.BuildElementTree(updatedFlat), nil
	})
	if err != nil {
		return nil, 0, err
	}
	return result, seq, nil
}

func (uc *elementUseCase) RemoveElement(ctx context.Context, projectID string, pageID string, elementID string) (int64, error) {
	return uc.docUseCase.MutateDocument(ctx, projectID, pageID, func(elements []domainEntity.Element) ([]domainEntity.Element, error) {
		newRoots := make([]domainEntity.Element, 0, len(elements))
		removed := false
		for _, r := range elements {
			if r.ID == elementID {
				removed = true
				continue
			}
			newRoots = append(newRoots, r)
		}
		if removed {
			return newRoots, nil
		}
		for i := range elements {
			if elementutils.DeleteElement(&elements[i], elementID) {
				return elements, nil
			}
		}
		return nil, domainErrors.New(domainErrors.CodeNotFound, "element not found", nil)
	})
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
