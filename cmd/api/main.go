package main

import (
	"context"
	"strings"
	ulogger "webbuilder/internal/utils/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"webbuilder/internal/config"
	"webbuilder/internal/delivery/http/middleware"
	"webbuilder/internal/delivery/ws/handlers"
	"webbuilder/internal/repository"
	"webbuilder/internal/usecase"
	"webbuilder/pkg/database"
	"webbuilder/pkg/redis"
)

func main() {
	lg := ulogger.Default()
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		lg.Fatalf("Failed to load config: %v", err)
	}
	if cfg.IsUnsafeDevMode() {
		lg.Fatalf("Refusing to start with DEV_MODE=true in env=%q or with live Clerk key", cfg.Env)
	}

	db, err := database.NewPostgresConnection(database.Config{
		URL:      cfg.Database.URL,
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
	// if err := db.AutoMigrate(
	// 	&domainEntity.Project{},
	// 	&domainEntity.Page{},
	// 	&domainEntity.Element{},

	// 	&domainEntity.Snapshot{},
	// 	&domainEntity.EventWorkflow{},
	// 	&domainEntity.ElementEventWorkflow{},
	// 	&domainEntity.ElementComment{},
	// 	&domainEntity.Comment{},
	// ); err != nil {
	// 	lg.Fatalf("Failed to migrate database: %v", err)
	// }

	rdb, err := redis.NewClient(redis.Config{
		URL:      cfg.Redis.URL,
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		lg.Fatalf("Failed to connect to redis: %v", err)
	}

	// Initialize the unified repository container
	repos := repository.New(db, rdb)

	// Initialize the central UseCase container
	useCases := usecase.New(repos, lg)

	// Start the background persistence worker using the initialized use case
	go useCases.Persist.Start(ctx)

	// Initialize WebSocket Handler with the central UseCase container
	wsHandler := handlers.NewHandler(useCases, rdb, lg)
	wsHandler.SetAccessChecker(useCases.Logic)
	wsHandler.SetPageChecker(repos.ElementRepo)
	wsHandler.SetAllowedOrigins(cfg.AllowedOrigins)

	app := fiber.New()

	app.Use(recover.New())
	app.Use(logger.New())

	corsCfg := cors.Config{
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}
	if len(cfg.AllowedOrigins) > 0 {
		corsCfg.AllowOrigins = strings.Join(cfg.AllowedOrigins, ",")
	} else {
		corsCfg.AllowOriginsFunc = func(origin string) bool {
			return false
		}
	}
	app.Use(cors.New(corsCfg))

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	app.Use(middleware.Authenticate(cfg, rdb))

	wsHandler.RegisterRoutes(app)

	lg.Infof("Server starting on port %s", cfg.ServerPort)
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		lg.Fatalf("Server failed to start: %v", err)
	}
}
