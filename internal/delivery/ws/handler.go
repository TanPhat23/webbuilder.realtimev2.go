package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	domainUsecase "webbuilder/internal/domain/usecase"
	"webbuilder/internal/utils/logger"
	"webbuilder/internal/utils/validation"
)

// MessageHandler defines the function signature for processing a specific message type.
type MessageHandler func(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error

// Handler manages WebSocket connections and delegates messages to specific command handlers.
type Handler struct {
	elementUseCase  domainUsecase.ElementUseCase
	documentUseCase domainUsecase.DocumentUseCase
	hub             *wsentity.Hub
	handlers        map[domainEntity.MessageType]MessageHandler
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	sendBuf         int
	logger          logger.Logger
}

// NewHandler initializes a new WebSocket handler with registered command handlers.
func NewHandler(uc domainUsecase.ElementUseCase, duc domainUsecase.DocumentUseCase, rdb *redis.Client, lg logger.Logger) *Handler {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Handler{
		elementUseCase:  uc,
		documentUseCase: duc,
		ctx:             ctx,
		cancel:          cancel,
		sendBuf:         64,
		logger:          lg,
	}
	h.hub = wsentity.NewHub(h.ctx, &h.wg, h.sendBuf, rdb, lg)
	h.registerHandlers()
	return h
}

// registerHandlers maps message types to their respective handling functions (Command Pattern).
func (h *Handler) registerHandlers() {
	h.handlers = map[domainEntity.MessageType]MessageHandler{
		domainEntity.TypeJoin:          h.handleJoin,
		domainEntity.TypeLeave:         h.handleLeave,
		domainEntity.TypeElementCreate: h.handleElementCreate,
		domainEntity.TypeElementUpdate: h.handleElementUpdate,
		domainEntity.TypeElementDelete: h.handleElementDelete,
		domainEntity.TypeElementMove:   h.handleElementMove,
		domainEntity.TypePresence:      h.handlePresence,
		domainEntity.TypeSync:          h.handleSync,
	}
}

// HandleWebSocket is the main entry point for upgraded WebSocket connections.
func (h *Handler) HandleWebSocket(c *websocket.Conn) {
	projectID := c.Params("project")
	client := wsentity.NewClient(c, projectID)

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

// --- Specific Command Handlers ---

func (h *Handler) handleJoin(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if msg.PageID == "" {
		return domainErrors.New(domainErrors.CodeValidation, "pageId is required to join", nil)
	}

	oldPage := client.PageID
	if oldPage != "" && oldPage != msg.PageID {
		_ = h.hub.Unregister(ctx, client)
	}

	client.SetPageID(msg.PageID)
	if err := h.hub.Register(ctx, client); err != nil {
		return err
	}

	// Send initial synchronization state
	elements, err := h.documentUseCase.GetDocument(ctx, client.ProjectID, client.PageID)
	if err == nil {
		syncMsg, _ := client.Factory().Sync(elements, nil)
		_ = client.Send(syncMsg)
	}
	return nil
}

func (h *Handler) handleLeave(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID != "" {
		_ = h.hub.Unregister(ctx, client)
		client.SetPageID("")
	}
	return nil
}

func (h *Handler) handleElementCreate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementCreate
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	payload.Element.PageID = client.PageID
	if err := validation.ValidateElementCreate(&payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.elementUseCase.AddElement(ctx, client.ProjectID, &payload.Element); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().ElementCreate(payload.Element)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementUpdate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementUpdate
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := validation.ValidateElementUpdate(&payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	// Use the specific PatchElement logic moved to use case for better scalability/reusability
	if err := h.elementUseCase.PatchElement(ctx, client.ProjectID, payload.ID, payload.Properties, payload.Styles, payload.Settings); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().ElementUpdate(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementDelete(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementDelete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.elementUseCase.RemoveElement(ctx, client.ProjectID, payload.ID); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().ElementDelete(payload.ID)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementMove(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementMove
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	parentID := ""
	if payload.ParentID != nil {
		parentID = *payload.ParentID
	}

	if _, err := h.elementUseCase.MoveElement(ctx, client.ProjectID, payload.ID, parentID, payload.Order); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().ElementMove(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handlePresence(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadPresence
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	broadcastMsg, _ := client.Factory().Presence(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleSync(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	elements, err := h.elementUseCase.SyncPage(ctx, client.ProjectID, client.PageID)
	if err != nil {
		return err
	}
	syncMsg, _ := client.Factory().Sync(elements, nil)
	_ = client.Send(syncMsg)
	return nil
}

// Shutdown gracefully stops the handler and waits for client writer goroutines to exit.
func (h *Handler) Shutdown() {
	h.cancel()
	h.wg.Wait()
}
