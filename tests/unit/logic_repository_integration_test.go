package unit

import (
	"context"
	"sync"
	"testing"

	"webbuilder/internal/repository"
)

func TestLogicRepositoryConcurrentNamesAndIsolation(t *testing.T) {
	_, db := legacyRealtimeDatabase(t)
	for _, sql := range []string{
		`CREATE TABLE "Project" ("Id" text PRIMARY KEY, "OwnerId" text NOT NULL, "DeletedAt" timestamp, "Logic" jsonb)`,
		`CREATE TABLE "Collaborator" ("ProjectId" text NOT NULL, "UserId" text NOT NULL, "Role" text NOT NULL)`,
		`CREATE TABLE "Page" ("Id" text PRIMARY KEY, "ProjectId" text NOT NULL, "Logic" jsonb)`,
		`INSERT INTO "Project" ("Id", "OwnerId") VALUES ('proj', 'owner')`,
		`INSERT INTO "Collaborator" ("ProjectId", "UserId", "Role") VALUES ('proj', 'viewer', 'viewer'), ('proj', 'editor', 'editor')`,
		`INSERT INTO "Page" ("Id", "ProjectId") VALUES ('p1', 'proj'), ('p2', 'proj')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.NewLogicRepository(db)
	for _, tc := range []struct {
		user        string
		read, write bool
	}{{"owner", true, true}, {"editor", true, true}, {"viewer", true, false}, {"stranger", false, false}} {
		read, write, err := repo.Access(context.Background(), "proj", tc.user)
		if err != nil || read != tc.read || write != tc.write {
			t.Fatalf("%s: access %v/%v: %v", tc.user, read, write, err)
		}
	}
	patch := map[string]any{"variables": map[string]any{"count": map[string]any{"type": "number", "default": float64(1)}}}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for _, id := range []string{"p1", "p2"} {
		wg.Add(1)
		go func(page string) {
			defer wg.Done()
			_, err := repo.Apply(context.Background(), "proj", page, "editor", patch)
			errors <- err
		}(id)
	}
	wg.Wait()
	close(errors)
	successes := 0
	for err := range errors {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one successful conflicting update, got %d", successes)
	}
	project, page, err := repo.Get(context.Background(), "proj", "p1", "viewer")
	if err != nil {
		t.Fatal(err)
	}
	if project["v"] != float64(1) || page["v"] != float64(1) {
		t.Fatal("snapshot did not include both documents")
	}
	if _, _, err := repo.Get(context.Background(), "other", "p1", "stranger"); err == nil {
		t.Fatal("cross-project read accepted")
	}
	if _, err := repo.Apply(context.Background(), "proj", "p1", "viewer", patch); err == nil {
		t.Fatal("viewer can write")
	}
	if _, _, err := repo.Get(context.Background(), "proj", "p1", "stranger"); err == nil {
		t.Fatal("stranger can read")
	}
	if _, err := repo.Apply(context.Background(), "other", "p1", "stranger", patch); err == nil {
		t.Fatal("cross-project update accepted")
	}
}
