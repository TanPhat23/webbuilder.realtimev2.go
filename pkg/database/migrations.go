package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateRealtimeSchema upgrades the API-owned schema only for fields required
// by the realtime engine. Do not AutoMigrate the full entity graph: the API and
// realtime service share tables and must not rewrite each other's definitions.
func migrateRealtimeSchema(db *gorm.DB) error {
	// An already-upgraded schema also works with a runtime role that cannot ALTER.
	if db.Migrator().HasColumn("Element", "OrderKey") {
		return nil
	}
	// IF NOT EXISTS is safe when multiple service instances start together.
	// Existing rows retain legacy numeric ordering until NormalizeOrder assigns
	// fractional keys, and API inserts that omit OrderKey still receive a default.
	if err := db.Exec(`ALTER TABLE "Element" ADD COLUMN IF NOT EXISTS "OrderKey" text NOT NULL DEFAULT ''`).Error; err != nil {
		return fmt.Errorf("migrate realtime element order keys: %w", err)
	}
	return nil
}
