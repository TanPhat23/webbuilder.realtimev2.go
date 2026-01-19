package usecase

import (
	"context"
	"webbuilder/internal/domain/entity"
)

/*
ElementUseCase contains operations related to element lifecycle.

Methods are intentionally documented at the interface to preserve the
public contract for implementations and to keep implementation files
comment-free per your request.
*/
type ElementUseCase interface {
	// AddElement creates a new element in the given project/page and refreshes the page cache.
	AddElement(ctx context.Context, projectID string, element *entity.Element) error
	// UpdateElement updates an existing element and refreshes the page cache.
	UpdateElement(ctx context.Context, projectID string, element *entity.Element) error
	// PatchElement applies granular updates to an element and refreshes the page cache.
	PatchElement(ctx context.Context, projectID string, elementID string, properties map[string]any, styles map[string]any, settings *entity.Setting) error
	// RemoveElement deletes an element and refreshes the page cache.
	RemoveElement(ctx context.Context, projectID string, elementID string) error
	// MoveElement moves the element under a new parent at the given index and refreshes the page cache.
	MoveElement(ctx context.Context, projectID string, elementID string, parentID string, index int) (*entity.Element, error)
	// GetElementsByPageID returns all elements that belong to the specified page.
	GetElementsByPageID(ctx context.Context, projectID string, pageID string) ([]entity.Element, error)
	// GetElementsByProjectID returns all elements across the specified project.
	GetElementsByProjectID(ctx context.Context, projectID string) ([]entity.Element, error)
	// SyncPage forces a refresh of the page cache from the DB and returns the latest elements.
	SyncPage(ctx context.Context, projectID string, pageID string) ([]entity.Element, error)
}

/*
DocumentUseCase contains operations related to page retrieval and cache management.

Method comments are kept here to document the exact responsibilities of the
DocumentUseCase so implementations elsewhere can remain comment-free.
*/
type DocumentUseCase interface {
	// GetDocument returns the cached snapshot for a page, or builds and returns it if not present.
	GetDocument(ctx context.Context, projectID string, pageID string) ([]entity.Element, error)
	// RefreshCache fetches elements for a page from the DB and updates the cache.
	RefreshCache(ctx context.Context, projectID string, pageID string) error
	// AddElementsToDocument sets the cached snapshot for the given page.
	AddElementsToDocument(ctx context.Context, projectID string, pageID string, elements []entity.Element) error
}
