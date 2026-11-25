package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"

	domainEntity "webbuilder/internal/domain/entity"
)

const (
	MaxElementDepth = 32
	MaxElementNodes = 2000
)

var (
	validate *validator.Validate
	safeIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// IsSafeID reports whether id is a safe 1..128 character identifier containing
// only ASCII letters, digits, underscores, and hyphens (no ':', slashes, whitespace, or control chars).
func IsSafeID(id string) bool {
	return safeIDRE.MatchString(id)
}

// ElementTree enforces depth (max 32) and total node count (max 2000) bounds on an element tree.
func ElementTree(e *domainEntity.Element) error {
	if e == nil {
		return nil
	}
	nodes := 0
	return validateElementNode(e, 1, &nodes)
}

func validateElementNode(e *domainEntity.Element, depth int, nodes *int) error {
	if depth > MaxElementDepth {
		return fmt.Errorf("element tree exceeds maximum depth of %d", MaxElementDepth)
	}
	*nodes++
	if *nodes > MaxElementNodes {
		return fmt.Errorf("element tree exceeds maximum node count of %d", MaxElementNodes)
	}
	for i := range e.Elements {
		if err := validateElementNode(&e.Elements[i], depth+1, nodes); err != nil {
			return err
		}
	}
	return nil
}

// CountElements returns the total number of nodes in a forest of elements.
func CountElements(elements []domainEntity.Element) int {
	count := 0
	for i := range elements {
		count += 1 + CountElements(elements[i].Elements)
	}
	return count
}

func init() {
	validate = validator.New()

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}

func Presence(p *domainEntity.PayloadPresence) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if p.UserID == "" {
		return fmt.Errorf("userId is required")
	}
	return nil
}

// ElementCreate validates the payload for adding a new element to the canvas
func ElementCreate(p *domainEntity.PayloadElementCreate) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	return ElementForCreate(&p.Element)
}

// ElementUpdate validates granular updates to properties or styles
func ElementUpdate(p *domainEntity.PayloadElementUpdate) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	return nil
}

// ElementMove validates reordering or reparenting an element
func ElementMove(p *domainEntity.PayloadElementMove) error {
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

// ElementDelete validates removal of an element
func ElementDelete(p *domainEntity.PayloadElementDelete) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	return nil
}

// ElementContent validates real-time text content updates
func ElementContent(p *domainEntity.PayloadElementContent) error {
	if p == nil {
		return fmt.Errorf("payload is nil")
	}
	if err := validate.Var(p.ID, "required,uuid"); err != nil {
		return fmt.Errorf("invalid element id: %w", err)
	}
	return nil
}

// Comment validates a new or updated comment
func Comment(p *domainEntity.PayloadComment) error {
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

// ElementForCreate validates an Element struct for creation
func ElementForCreate(e *domainEntity.Element) error {
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
	if !IsSafeID(e.PageID) {
		return fmt.Errorf("invalid pageId")
	}
	if err := ElementTree(e); err != nil {
		return err
	}
	var savedParent *string
	if e.ParentID != nil {
		// Treat an empty string or the special 'root' identifier as 'no parent'
		// for validation purposes so that clients can reference 'root' without
		// failing the UUID validation rule.
		v := strings.TrimSpace(*e.ParentID)
		if v == "" || v == "root" {
			savedParent = e.ParentID
			e.ParentID = nil
		}
	}
	if err := validate.Struct(e); err != nil {
		// restore before returning
		if savedParent != nil {
			e.ParentID = savedParent
		}
		return fmt.Errorf("invalid element for add: %w", err)
	}
	if savedParent != nil {
		e.ParentID = savedParent
	}
	return nil
}

func ElementForUpdate(e *domainEntity.Element) error {
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
