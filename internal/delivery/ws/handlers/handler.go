package handlers

import (
	"context"
	"errors"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	domainUsecase "webbuilder/internal/domain/usecase"
	"webbuilder/internal/utils/logger"
)

// MessageHandler defines the function signature for processing a specific message type.
type MessageHandler func(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error

// Handler manages WebSocket connections and delegates messages to specific command handlers.
type Handler struct {
	uc       *domainUsecase.UseCase
	hub      *wsentity.Hub
	handlers map[domainEntity.MessageType]MessageHandler
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	sendBuf  int
	logger   logger.Logger
}

// NewHandler initializes a new WebSocket handler with registered command handlers.
func NewHandler(uc *domainUsecase.UseCase, rdb *redis.Client, lg logger.Logger) *Handler {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Handler{
		uc:      uc,
		ctx:     ctx,
		cancel:  cancel,
		sendBuf: 64,
		logger:  lg,
	}
	h.hub = wsentity.NewHub(h.ctx, &h.wg, h.sendBuf, rdb, lg)
	h.registerHandlers()
	return h
}

// registerHandlers maps message types to their respective handling functions (Command Pattern).
func (h *Handler) registerHandlers() {
	h.handlers = map[domainEntity.MessageType]MessageHandler{
		// Presence & Sync
		domainEntity.TypeJoin:     h.handleJoin,
		domainEntity.TypeLeave:    h.handleLeave,
		domainEntity.TypePresence: h.handlePresence,
		domainEntity.TypeSync:     h.handleSync,

		// Element Operations
		domainEntity.TypeElementCreate: h.handleElementCreate,
		domainEntity.TypeElementUpdate: h.handleElementUpdate,
		domainEntity.TypeElementDelete: h.handleElementDelete,
		domainEntity.TypeElementMove:   h.handleElementMove,

		// Workflow Operations
		domainEntity.TypeWorkflowCreate:        h.handleWorkflowCreate,
		domainEntity.TypeWorkflowUpdate:        h.handleWorkflowUpdate,
		domainEntity.TypeWorkflowDelete:        h.handleWorkflowDelete,
		domainEntity.TypeElementWorkflowCreate: h.handleElementWorkflowCreate,
		domainEntity.TypeElementWorkflowUpdate: h.handleElementWorkflowUpdate,
		domainEntity.TypeElementWorkflowDelete: h.handleElementWorkflowDelete,
	}
}

// HandleWebSocket is the main entry point for upgraded WebSocket connections.
func (h *Handler) HandleWebSocket(c *websocket.Conn) {
	projectID := c.Params("project")
	user, _ := c.Locals("user").(*domainEntity.User)
	client := wsentity.NewClient(c, projectID, user)

	if projectID == "" {
		errMsg := client.Factory().Error(domainErrors.CodeValidation, "project ID is required")
		_ = client.Conn.WriteJSON(errMsg)
		client.Close()
		return
	}

	ctx := context.Background()

	defer func() {
		_ = h.hub.Unregister(h.ctx, client)
	}()

	for {
		var msg domainEntity.Message
		if err := c.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Errorf("websocket error: %v", err)
			}
			break
		}

		if err := h.processMessage(ctx, client, msg); err != nil {
			h.logger.Errorf("Error processing message %s: %v", msg.Type, err)

			// Extract structured error code if available
			errCode := domainErrors.CodeProcessError
			var appErr *domainErrors.AppError
			if errors.As(err, &appErr) {
				errCode = appErr.Code
			}

			errMsg := client.Factory().Error(errCode, err.Error())
			if err := client.Send(errMsg); err != nil {
				h.logger.Errorf("failed to send error to client: %v", err)
			}
		}
	}
}

// processMessage resolves the handler for a message and executes it.
func (h *Handler) processMessage(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	// Ensure consistency in the incoming message metadata
	msg.ProjectID = client.ProjectID
	if msg.PageID == "" {
		msg.PageID = client.PageID
	}

	handler, ok := h.handlers[msg.Type]
	if !ok {
		return domainErrors.New(domainErrors.CodeValidation, "unsupported message type", nil)
	}

	return handler(ctx, client, msg)
}

// Shutdown gracefully stops the handler and waits for client writer goroutines to exit.
func (h *Handler) Shutdown() {
	h.cancel()
	h.wg.Wait()
}