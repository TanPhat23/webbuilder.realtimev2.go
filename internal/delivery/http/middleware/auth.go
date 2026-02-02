package middleware

import (
	"os"
	"strings"

	"webbuilder/internal/config"
	"webbuilder/internal/domain/entity"
	"webbuilder/internal/utils/logger"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Handle DevMode from application configuration
		if cfg != nil && cfg.DevMode {
			devUserId := os.Getenv("DEV_USER_ID")
			if devUserId == "" {
				devUserId = "dev-user-id"
			}

			logger.Default().Debugf("AuthMiddleware: DevMode active, using UserID: %s", devUserId)

			user := &entity.User{
				ID:    devUserId,
				Email: "dev@example.com",
			}
			c.Locals("userId", user.ID)
			c.Locals("user", user)
			return c.Next()
		}

		var token string

		// 2. Check Authorization header: Bearer <token>
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				token = parts[1]
			}
		}

		// 3. Check query parameter: ?token=<token>
		if token == "" {
			token = c.Query("token")
		}

		// 4. Validate token presence
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Missing or invalid token",
			})
		}

		// TODO: Implement actual token validation (JWT/Session) here.
		// For now, we use a placeholder user for authenticated requests.
		user := &entity.User{
			ID:    "test-user-id",
			Email: "test@example.com",
		}

		c.Locals("userId", user.ID)
		c.Locals("user", user)

		return c.Next()
	}
}
