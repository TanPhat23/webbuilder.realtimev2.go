package handlers

import (
	"strings"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// SetAllowedOrigins restricts WebSocket upgrade requests to the given Origin values.
// Empty Origin headers are permitted for non-browser clients when origins are configured.
func (h *Handler) SetAllowedOrigins(origins []string) {
	cleaned := make([]string, 0, len(origins))
	for _, o := range origins {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	h.allowedOrigins = cleaned
}

func (h *Handler) SetupRoutes(app *fiber.App) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	wsCfg := websocket.Config{}
	if len(h.allowedOrigins) > 0 {
		origins := append([]string(nil), h.allowedOrigins...)
		hasWildcard := false
		hasEmpty := false
		for _, o := range origins {
			if o == "*" {
				hasWildcard = true
			}
			if o == "" {
				hasEmpty = true
			}
		}
		if !hasWildcard && !hasEmpty {
			origins = append(origins, "")
		}
		wsCfg.Origins = origins
	}

	app.Get("/ws/:project", websocket.New(h.HandleWebSocket, wsCfg))
}
