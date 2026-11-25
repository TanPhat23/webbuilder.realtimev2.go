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

const (
	msgUnsafeDevMode  = "Server misconfigured: DEV_MODE is forbidden in this environment"
	msgMissingTicket  = "Unauthorized: Missing or invalid WebSocket ticket"
	msgNoTicketStore  = "Server misconfigured: missing ticket store"
	msgInvalidTicket  = "Unauthorized: Invalid or expired ticket"
	msgInvalidPayload = "Unauthorized: Invalid ticket payload"
)

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
		return func(c *fiber.Ctx) error { return abort(c, fiber.StatusInternalServerError, msgUnsafeDevMode) }
	}

	userClient := newClerkUserClient(cfg)

	return func(c *fiber.Ctx) error {
		if cfg != nil && cfg.IsUnsafeDevMode() {
			return abort(c, fiber.StatusInternalServerError, msgUnsafeDevMode)
		}
		if cfg != nil && cfg.DevMode {
			return authenticateDevUser(c)
		}
		return authenticateWSTicket(c, redisClient, userClient)
	}
}

func abort(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

func newClerkUserClient(cfg *config.Config) *user.Client {
	if cfg == nil || cfg.ClerkSecretKey == "" {
		return nil
	}
	clerkCfg := &clerk.ClientConfig{}
	clerkCfg.Key = clerk.String(cfg.ClerkSecretKey)
	return user.NewClient(clerkCfg)
}

// proceed stores the authenticated user for downstream handlers.
func proceed(c *fiber.Ctx, u *entity.User) error {
	c.Locals("userId", u.ID)
	c.Locals("user", u)
	return c.Next()
}

func authenticateDevUser(c *fiber.Ctx) error {
	devUserID := os.Getenv("DEV_USER_ID")
	if devUserID == "" {
		devUserID = "dev-user-id"
	}
	// Dev mode only: let a test client pick its identity, so several
	// users can connect at once.
	if override := c.Query("devUser", c.Get("X-Dev-User")); override != "" {
		devUserID = override
	}
	logger.Default().Debugf("AuthMiddleware: DevMode active, using UserID: %s", devUserID)
	return proceed(c, &entity.User{ID: devUserID, Email: "dev@example.com"})
}

func authenticateWSTicket(c *fiber.Ctx, rdb *redis.Client, userClient *user.Client) error {
	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket == "" {
		return abort(c, fiber.StatusUnauthorized, msgMissingTicket)
	}
	if rdb == nil {
		return abort(c, fiber.StatusInternalServerError, msgNoTicketStore)
	}
	if !validation.IsValidSafeID(ticket) {
		return abort(c, fiber.StatusUnauthorized, msgInvalidTicket)
	}

	sum := sha256.Sum256([]byte(ticket))
	value, err := rdb.GetDel(c.UserContext(), wsTicketRedisPrefix+hex.EncodeToString(sum[:])).Result()
	value = strings.TrimSpace(value)
	if err != nil || value == "" {
		return abort(c, fiber.StatusUnauthorized, msgInvalidTicket)
	}

	if strings.HasPrefix(value, "{") {
		return authenticateTicketPayload(c, userClient, value)
	}
	mapped := &entity.User{ID: value}
	if userClient != nil {
		enrichClerkUser(c.UserContext(), userClient, mapped)
	}
	return proceed(c, mapped)
}

type wsTicketPayload struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`
	ProjectID string  `json:"projectId"`
	Email     string  `json:"email"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	ImageUrl  *string `json:"imageUrl"`
}

func authenticateTicketPayload(c *fiber.Ctx, userClient *user.Client, value string) error {
	var payload wsTicketPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return abort(c, fiber.StatusUnauthorized, msgInvalidPayload)
	}
	uid := payload.UserID
	if uid == "" {
		uid = payload.ID
	}
	if uid == "" {
		return abort(c, fiber.StatusUnauthorized, msgInvalidPayload)
	}
	mapped := &entity.User{
		ID:        uid,
		Email:     payload.Email,
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		ImageUrl:  payload.ImageUrl,
	}
	if userClient != nil && mapped.Email == "" {
		enrichClerkUser(c.UserContext(), userClient, mapped)
	}
	if payload.ProjectID != "" {
		c.Locals("ticketProjectId", payload.ProjectID)
	}
	return proceed(c, mapped)
}

type clerkUserPayload struct {
	EmailAddresses []struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"emailAddresses"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	ImageURL  string `json:"imageUrl"`
}

func enrichClerkUser(ctx context.Context, userClient *user.Client, mapped *entity.User) {
	usr, err := userClient.Get(ctx, mapped.ID)
	if err != nil {
		logger.Default().Debugf("AuthMiddleware: failed to fetch user from Clerk: %v", err)
		return
	}
	raw, err := json.Marshal(usr)
	if err != nil {
		return
	}
	var payload clerkUserPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	applyClerkUserFields(mapped, payload)
}

func applyClerkUserFields(mapped *entity.User, p clerkUserPayload) {
	if len(p.EmailAddresses) > 0 && p.EmailAddresses[0].EmailAddress != "" {
		mapped.Email = p.EmailAddresses[0].EmailAddress
	}
	setIfPresent(&mapped.FirstName, p.FirstName)
	setIfPresent(&mapped.LastName, p.LastName)
	setIfPresent(&mapped.ImageUrl, p.ImageURL)
}

func setIfPresent(dst **string, v string) {
	if v != "" {
		*dst = &v
	}
}
