package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	domainEntity "webbuilder/internal/domain/entity"
	domainRepo "webbuilder/internal/domain/repository"
)

const dirtyPageKey = "dirty_pages"

type documentRepository struct {
	client *redis.Client
	ttl    time.Duration
}

// NewDocumentRepository creates a new instance of DocumentRepository using Redis as storage
func NewDocumentRepository(client *redis.Client) domainRepo.DocumentRepository {
	return &documentRepository{
		client: client,
		ttl:    24 * time.Hour,
	}
}

// SetDocument stores the list of elements for a page in Redis
func (r *documentRepository) SetDocument(ctx context.Context, projectID string, pageID string, elements []domainEntity.Element) error {
	key := r.getKey(projectID, pageID)

	data, err := json.Marshal(elements)
	if err != nil {
		return fmt.Errorf("failed to marshal elements: %w", err)
	}

	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set value in redis: %w", err)
	}

	return nil
}

// GetDocument retrieves the list of elements for a page from Redis
func (r *documentRepository) GetDocument(ctx context.Context, projectID string, pageID string) ([]domainEntity.Element, error) {
	key := r.getKey(projectID, pageID)

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get value from redis: %w", err)
	}

	var elements []domainEntity.Element
	if err := json.Unmarshal([]byte(val), &elements); err != nil {
		return nil, fmt.Errorf("failed to unmarshal elements: %w", err)
	}

	return elements, nil
}

// DeleteDocument removes the page elements from Redis
func (r *documentRepository) DeleteDocument(ctx context.Context, projectID string, pageID string) error {
	key := r.getKey(projectID, pageID)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete key from redis: %w", err)
	}

	return nil
}

func (r *documentRepository) MarkPageDirty(ctx context.Context, projectID string, pageID string) error {
	val := fmt.Sprintf("%s:%s", projectID, pageID)
	added, err := r.client.SAdd(ctx, dirtyPageKey, val).Result()
	if err != nil {
		// Debug log for troubleshooting SAdd failures.
		fmt.Printf("[DEBUG] documentRepository.MarkPageDirty: SAdd failed key=%s member=%s err=%v\n", dirtyPageKey, val, err)
		return err
	}
	// Debug log showing SAdd result (0 = already present, >0 = new member added)
	fmt.Printf("[DEBUG] documentRepository.MarkPageDirty: SAdd key=%s member=%s added=%d\n", dirtyPageKey, val, added)
	return nil
}

func (r *documentRepository) PopDirtyPages(ctx context.Context, limit int) ([]domainEntity.DirtyPage, error) {
	results, err := r.client.SPopN(ctx, dirtyPageKey, int64(limit)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var dirtyPages []domainEntity.DirtyPage
	for _, result := range results {
		parts := strings.SplitN(result, ":", 2)
		if len(parts) == 2 {
			dirtyPages = append(dirtyPages, domainEntity.DirtyPage{
				ProjectID: parts[0],
				PageID:    parts[1],
			})
		}
	}
	return dirtyPages, nil
}

func (r *documentRepository) getKey(projectID string, pageID string) string {
	return fmt.Sprintf("page:%s:%s", projectID, pageID)
}
