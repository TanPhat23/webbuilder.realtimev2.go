package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	clerk "github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/redis/go-redis/v9"

	"webbuilder/internal/config"
	"webbuilder/internal/domain/entity"
	"webbuilder/internal/utils/logger"
	"webbuilder/internal/utils/validation"

	"github.com/gofiber/fiber/v2"
)

const wsTicketRedisPrefix = "ws:ticket:"

// AuthMiddleware authenticates /ws upgrade requests exclusively via single-use Redis ?ticket=
// credentials (or DEV_MODE in non-production environments).
func AuthMiddleware(cfg *config.Config, rdb ...*redis.Client) fiber.Handler {
	var redisClient *redis.Client
	if len(rdb) > 0 {
		redisClient = rdb[0]
	}

	// Fail closed if DEV_MODE is enabled in production/staging or with a live Clerk key.
	if cfg != nil && cfg.IsUnsafeDevMode() {
		logger.Default().Errorf("AuthMiddleware: DEV_MODE is forbidden in production/staging or with live Clerk key")
		return func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Server misconfigured: DEV_MODE is forbidden in this environment",
			})
		}
	}

	var userClient *user.Client
	if cfg != nil && cfg.ClerkSecretKey != "" {
		clerkCfg := &clerk.ClientConfig{}
		clerkCfg.Key = clerk.String(cfg.ClerkSecretKey)
		userClient = user.NewClient(clerkCfg)
	}

	return func(c *fiber.Ctx) error {
		if cfg != nil && cfg.IsUnsafeDevMode() {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Server misconfigured: DEV_MODE is forbidden in this environment",
			})
		}

		if cfg != nil && cfg.DevMode {
			devUserId := os.Getenv("DEV_USER_ID")
			if devUserId == "" {
				devUserId = "dev-user-id"
			}
			// Dev mode only: let a test client pick its identity, so several
			// users can connect at once.
			if override := c.Query("devUser", c.Get("X-Dev-User")); override != "" {
				devUserId = override
			}

			logger.Default().Debugf("AuthMiddleware: DevMode active, using UserID: %s", devUserId)

			devUser := &entity.User{
				ID:    devUserId,
				Email: "dev@example.com",
			}
			c.Locals("userId", devUser.ID)
			c.Locals("user", devUser)
			return c.Next()
		}

		ticket := strings.TrimSpace(c.Query("ticket"))
		if ticket == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Missing or invalid WebSocket ticket",
			})
		}
		if redisClient == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Server misconfigured: missing ticket store",
			})
		}
		if !validation.IsValidSafeID(ticket) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Invalid or expired ticket",
			})
		}

		sum := sha256.Sum256([]byte(ticket))
		ticketVal, err := redisClient.GetDel(c.UserContext(), wsTicketRedisPrefix+hex.EncodeToString(sum[:])).Result()
		if err != nil || strings.TrimSpace(ticketVal) == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Invalid or expired ticket",
			})
		}
		ticketVal = strings.TrimSpace(ticketVal)

		if strings.HasPrefix(ticketVal, "{") {
			var ticketUser struct {
				ID        string  `json:"id"`
				UserID    string  `json:"userId"`
				ProjectID string  `json:"projectId"`
				Email     string  `json:"email"`
				FirstName *string `json:"firstName"`
				LastName  *string `json:"lastName"`
				ImageUrl  *string `json:"imageUrl"`
			}
			if err := json.Unmarshal([]byte(ticketVal), &ticketUser); err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Unauthorized: Invalid ticket payload",
				})
			}
			uid := ticketUser.UserID
			if uid == "" {
				uid = ticketUser.ID
			}
			if uid == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Unauthorized: Invalid ticket payload",
				})
			}
			mappedUser := &entity.User{
				ID:        uid,
				Email:     ticketUser.Email,
				FirstName: ticketUser.FirstName,
				LastName:  ticketUser.LastName,
				ImageUrl:  ticketUser.ImageUrl,
			}
			if userClient != nil && mappedUser.Email == "" {
				enrichClerkUser(c.UserContext(), userClient, mappedUser)
			}
			c.Locals("userId", mappedUser.ID)
			c.Locals("user", mappedUser)
			if ticketUser.ProjectID != "" {
				c.Locals("ticketProjectId", ticketUser.ProjectID)
			}
			return c.Next()
		}

		mappedUser := &entity.User{ID: ticketVal}
		if userClient != nil {
			enrichClerkUser(c.UserContext(), userClient, mappedUser)
		}
		c.Locals("userId", mappedUser.ID)
		c.Locals("user", mappedUser)
		return c.Next()
	}
}

func enrichClerkUser(ctx context.Context, userClient *user.Client, mappedUser *entity.User) {
	usr, err := userClient.Get(ctx, mappedUser.ID)
	if err != nil {
		logger.Default().Debugf("AuthMiddleware: failed to fetch user from Clerk: %v", err)
		return
	}
	var raw map[string]any
	if b, err := json.Marshal(usr); err == nil {
		if err := json.Unmarshal(b, &raw); err == nil {
			if v, ok := raw["emailAddresses"]; ok {
				if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
					if e0, ok := arr[0].(map[string]interface{}); ok {
						if ea, ok := e0["emailAddress"].(string); ok && ea != "" {
							mappedUser.Email = ea
						}
					}
				}
			}
			if v, ok := raw["firstName"].(string); ok && v != "" {
				mappedUser.FirstName = new(string)
				*mappedUser.FirstName = v
			}
			if v, ok := raw["lastName"].(string); ok && v != "" {
				mappedUser.LastName = new(string)
				*mappedUser.LastName = v
			}
			if v, ok := raw["imageUrl"].(string); ok && v != "" {
				mappedUser.ImageUrl = new(string)
				*mappedUser.ImageUrl = v
			}
		}
	}
}
