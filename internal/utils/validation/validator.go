package validation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	domainEntity "webbuilder/internal/domain/entity"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Use the `json` tag name in error messages where applicable.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}

// ValidatePresence validates user presence info (cursor, active element)
func ValidatePresence(p *domainEntity.PayloadPresence) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if p.UserID == "" {
		return fmt.Errorf("userId is required")
	}
	return nil
}

// ValidateElementCreate validates the payload for adding a new element to the canvas
func ValidateElementCreate(p *domainEntity.PayloadElementCreate) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	return ValidateElementForAdd(&p.Element)
}

// ValidateElementUpdate validates granular updates to properties or styles
func ValidateElementUpdate(p *domainEntity.PayloadElementUpdate) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	return nil
}

// ValidateElementMove validates reordering or reparenting an element
func ValidateElementMove(p *domainEntity.PayloadElementMove) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	if p.Order < 0 {
		return fmt.Errorf("order must be non-negative")
	}
	if p.ParentID != nil && *p.ParentID != "" {
		if err := validate.Var(*p.ParentID, "uuid"); err != nil {
			return fmt.Errorf("invalid parentId: %w", err)
		}
	}
	return nil
}

// ValidateElementDelete validates removal of an element
func ValidateElementDelete(p *domainEntity.PayloadElementDelete) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	return nil
}

// ValidateElementContent validates real-time text content updates
func ValidateElementContent(p *domainEntity.PayloadElementContent) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	return nil
}

// ValidateComment validates a new or updated comment
func ValidateComment(p *domainEntity.PayloadComment) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ElementID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	if p.Comment.Content == "" {
		return fmt.Errorf("content is required")
	}
	return nil
}

// ValidateElementForAdd validates an Element struct for creation
func ValidateElementForAdd(e *domainEntity.Element) error {
	if e == nil {
		return fmt.Errorf("element is nil")
	}
	// We check core fields for creation
	if e.Type == "" {
		return fmt.Errorf("element type is required")
	}
	if e.PageID == "" {
		return fmt.Errorf("pageId is required")
	}
	if err := validate.Struct(e); err != nil {
		return fmt.Errorf("invalid element for add: %w", err)
	}
	return nil
}

// ValidateElementForUpdate validates an Element struct for full updates (legacy/fallback)
func ValidateElementForUpdate(e *domainEntity.Element) error {
	if e == nil {
		return fmt.Errorf("element is nil")
	}
	if err := validate.Var(e.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	// Partial validation for updates
	if err := validate.StructPartial(e, "Type", "Order"); err != nil {
		return fmt.Errorf("invalid element for update: %w", err)
	}
	return nil
}
