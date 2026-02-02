package seed

import (
	"log"
	"time"
	"webbuilder/internal/domain/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SeedData struct {
	db *gorm.DB
}

func NewSeedData(db *gorm.DB) *SeedData {
	return &SeedData{db: db}
}

// Run performs a full seeding of the database in the correct order
func (s *SeedData) Run() error {
	log.Println("Starting full seeding process...")

	if err := SeedUsers(s.db); err != nil {
		return err
	}
	if err := SeedProjects(s.db); err != nil {
		return err
	}
	if err := SeedWorkflows(s.db); err != nil {
		return err
	}

	log.Println("Full seeding completed successfully.")
	return nil
}

// SeedUsers seeds basic user data for testing
func SeedUsers(db *gorm.DB) error {
	email := "test@example.com"
	var found int64
	db.Model(&entity.User{}).Where("email = ?", email).Count(&found)
	if found > 0 {
		log.Println("User already exists, skipping seeding.")
		return nil
	}

	user := &entity.User{
		ID:        uuid.New().String(),
		Email:     "test@example.com",
		FirstName: stringPtr("Test"),
		LastName:  stringPtr("User"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.FirstOrCreate(user, entity.User{Email: user.Email}).Error; err != nil {
		log.Printf("Error seeding user: %v", err)
		return err
	}
	log.Printf("User seeded: %s", user.Email)
	return nil
}

// SeedProjects seeds a sample project along with its pages and elements
func SeedProjects(db *gorm.DB) error {
	var user entity.User
	if err := db.First(&user).Error; err != nil {
		log.Println("No user found, seeding default user first...")
		if err := SeedUsers(db); err != nil {
			return err
		}
		db.First(&user)
	}

	project := &entity.Project{
		ID:          uuid.New().String(),
		Name:        "Test Project",
		Description: stringPtr("A sample project for testing purposes"),
		OwnerID:     user.ID,
		Published:   false,
		Subdomain:   stringPtr("test-project"),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Styles:      map[string]any{"backgroundColor": "#ffffff"},
	}

	if err := db.Create(project).Error; err != nil {
		log.Printf("Error seeding project: %v", err)
		return err
	}
	log.Printf("Project seeded: %s", project.Name)

	return SeedPages(db, project.ID)
}

// SeedPages seeds a home page for a specific project
func SeedPages(db *gorm.DB, projectID string) error {
	page := &entity.Page{
		ID:        uuid.New().String(),
		Name:      "Home",
		Type:      "HOME",
		ProjectID: projectID,
		Styles:    map[string]any{"padding": "20px"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.Create(page).Error; err != nil {
		log.Printf("Error seeding page: %v", err)
		return err
	}
	log.Printf("Page seeded: %s", page.Name)

	return SeedElements(db, page.ID)
}

// SeedElements seeds a set of basic UI elements for a page
func SeedElements(db *gorm.DB, pageID string) error {
	containerID := uuid.New().String()
	elements := []entity.Element{
		{
			ID:     containerID,
			Type:   "container",
			PageID: pageID,
			Order:  0,
			Styles: map[string]any{"display": "flex", "flexDirection": "column"},
		},
		{
			ID:       uuid.New().String(),
			Type:     "text",
			Content:  stringPtr("Welcome to WebBuilder"),
			PageID:   pageID,
			ParentID: &containerID,
			Order:    0,
			Styles:   map[string]any{"fontSize": "24px", "fontWeight": "bold"},
		},
		{
			ID:       uuid.New().String(),
			Type:     "button",
			Content:  stringPtr("Click Me"),
			PageID:   pageID,
			ParentID: &containerID,
			Order:    1,
			Styles:   map[string]any{"backgroundColor": "#007bff", "color": "#ffffff"},
		},
	}

	for _, el := range elements {
		if err := db.Create(&el).Error; err != nil {
			log.Printf("Error seeding element %s: %v", el.Type, err)
			return err
		}
		log.Printf("Element seeded: %s", el.Type)
	}
	return nil
}

// SeedWorkflows seeds a sample workflow and attaches it to a button element
func SeedWorkflows(db *gorm.DB) error {
	var project entity.Project
	if err := db.First(&project).Error; err != nil {
		log.Println("No project found, seeding project first...")
		if err := SeedProjects(db); err != nil {
			return err
		}
		db.First(&project)
	}

	workflow := &entity.EventWorkflow{
		ID:          uuid.New().String(),
		ProjectID:   project.ID,
		Name:        "Alert Workflow",
		Description: stringPtr("Shows an alert when triggered"),
		Enabled:     true,
		Handlers: []any{
			map[string]any{
				"type": "action",
				"action": map[string]any{
					"name": "alert",
					"params": map[string]any{
						"message": "Hello from Workflow!",
					},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.Create(workflow).Error; err != nil {
		log.Printf("Error seeding workflow: %v", err)
		return err
	}
	log.Printf("Workflow seeded: %s", workflow.Name)

	// Try to attach to the first button element found in the DB
	var button entity.Element
	if err := db.Where("type = ?", "button").First(&button).Error; err == nil {
		ew := &entity.ElementEventWorkflow{
			ID:         uuid.New().String(),
			ElementID:  button.ID,
			WorkflowID: workflow.ID,
			EventName:  "onClick",
			CreatedAt:  time.Now(),
		}
		if err := db.Create(ew).Error; err == nil {
			log.Printf("Attached workflow to button %s (event: onClick)", button.ID)
		}
	}

	return nil
}

func stringPtr(s string) *string {
	return &s
}
