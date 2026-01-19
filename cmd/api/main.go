package main

import (
	ulogger "webbuilder/internal/utils/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"webbuilder/internal/config"
	"webbuilder/internal/delivery/ws"
	domainEntity "webbuilder/internal/domain/entity"
	"webbuilder/internal/repository"
	"webbuilder/internal/usecase"
	"webbuilder/pkg/database"
	"webbuilder/pkg/redis"
)

func main() {
	lg := ulogger.Default()
	cfg, err := config.LoadConfig()
	if err != nil {
		lg.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewPostgresConnection(database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.Name,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		lg.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto Migrate
	if err := db.AutoMigrate(
		&domainEntity.Project{},
		&domainEntity.Page{},
		&domainEntity.Element{},
		&domainEntity.Setting{},
		&domainEntity.Snapshot{},
		&domainEntity.EventWorkflow{},
		&domainEntity.ElementEventWorkflow{},
		&domainEntity.ElementComment{},
		&domainEntity.Comment{},
	); err != nil {
		lg.Fatalf("Failed to migrate database: %v", err)
	}

	rdb, err := redis.NewRedisClient(redis.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		lg.Fatalf("Failed to connect to redis: %v", err)
	}

	// Initialize the unified repository
	repos := repository.NewRepository(db, rdb)

	// Initialize Use Cases
	docUseCase := usecase.NewDocumentUseCase(repos, repos)
	elemUseCase := usecase.NewElementUseCase(repos, docUseCase)

	// Initialize WebSocket Handler
	wsHandler := ws.NewHandler(elemUseCase, docUseCase, rdb, lg)

	app := fiber.New()

	app.Use(logger.New())
	app.Use(cors.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	wsHandler.SetupRoutes(app)

	lg.Infof("Server starting on port %s", cfg.ServerPort)
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		lg.Fatalf("Server failed to start: %v", err)
	}
}
