package handlers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	domainUsecase "webbuilder/internal/domain/usecase"
	"webbuilder/internal/utils/logger"
	"webbuilder/internal/utils/validation"
)

const (
	maxWSMessageBytes  = 512 << 10 // 512 KB
	wsPongWait         = 60 * time.Second
	wsPingPeriod       = (wsPongWait * 9) / 10
	wsWriteWait        = 10 * time.Second
	wsRateCapacity     = 60.0
	wsRateRefillPerSec = 30.0
)

type MessageHandler func(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error

type Handler struct {
	uc             *domainUsecase.UseCase
	hub            *wsentity.Hub
	handlers       map[domainEntity.MessageType]MessageHandler
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	sendBuf        int
	logger         logger.Logger
	pages          *pageActors
	allowedOrigins []string

	access      ProjectAccess
	pageChecker PageChecker

	// activeConns tracks the currently active client per user+project so that
	// duplicate connections (e.g. rapid reconnects) can be cleaned up.
	activeConnsMu sync.Mutex
	activeConns   map[string]*wsentity.Client
}

// ProjectAccess reports a user's permissions on a project.
type ProjectAccess interface {
	Access(ctx context.Context, projectID, userID string) (read bool, write bool, err error)
}

// PageChecker verifies whether a page belongs to a project.
type PageChecker interface {
	PageBelongsToProject(ctx context.Context, projectID, pageID string) (bool, error)
}

// RequireAccess makes the handler refuse users with no access to a project and
// reject every mutating message from read-only collaborators.
func (h *Handler) RequireAccess(a ProjectAccess) { h.access = a }

// RequirePageCheck makes handleJoin verify that the requested page belongs to the project.
func (h *Handler) RequirePageCheck(p PageChecker) { h.pageChecker = p }

// mutating lists the message types that change project data.
var mutating = map[domainEntity.MessageType]bool{
	domainEntity.TypeElementCreate:         true,
	domainEntity.TypeElementUpdate:         true,
	domainEntity.TypeElementDelete:         true,
	domainEntity.TypeElementMove:           true,
	domainEntity.TypeElementContent:        true,
	domainEntity.TypeCommentCreate:         true,
	domainEntity.TypeCommentUpdate:         true,
	domainEntity.TypeCommentDelete:         true,
	domainEntity.TypeLogicUpdate:           true,
	domainEntity.TypeLogicMove:             true,
	domainEntity.TypeSnapshot:              true,
	domainEntity.TypeWorkflowCreate:        true,
	domainEntity.TypeWorkflowUpdate:        true,
	domainEntity.TypeWorkflowDelete:        true,
	domainEntity.TypeElementWorkflowCreate: true,
	domainEntity.TypeElementWorkflowUpdate: true,
	domainEntity.TypeElementWorkflowDelete: true,
}

// connKey returns a unique key for tracking a user's active connection to a project.
func connKey(projectID, userID string) string {
	return projectID + "::" + userID
}

func NewHandler(uc *domainUsecase.UseCase, rdb *redis.Client, lg logger.Logger) *Handler {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Handler{
		uc:          uc,
		ctx:         ctx,
		cancel:      cancel,
		sendBuf:     64,
		logger:      lg,
		activeConns: make(map[string]*wsentity.Client),
	}
	h.pages = newPageActors(ctx, pageActorIdle)
	h.hub = wsentity.NewHub(h.ctx, &h.wg, h.sendBuf, rdb, lg)
	h.registerHandlers()
	return h
}

// registerHandlers maps message types to their respective handling functions (Command Pattern).
func (h *Handler) registerHandlers() {
	h.handlers = map[domainEntity.MessageType]MessageHandler{
		domainEntity.TypeLogicUpdate: h.handleLogicUpdate,
		domainEntity.TypeLogicMove:   h.handleLogicMove,
		domainEntity.TypeLogicSync:   h.handleLogicSync,

		// Presence & Sync
		domainEntity.TypeJoin:        h.handleJoin,
		domainEntity.TypeLeave:       h.handleLeave,
		domainEntity.TypePresence:    h.handlePresence,
		domainEntity.TypeSync:        h.handleSync,
		domainEntity.TypeSyncProject: h.handleSyncProject,

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
	if user == nil || user.ID == "" {
		h.logger.Errorf("WebSocket connection rejected: missing authenticated user (projectID=%s)", projectID)
		client := wsentity.NewClient(c, projectID, nil)
		_ = client.WriteJSON(client.Factory().Error(domainErrors.CodeUnauthorized, "unauthorized"))
		client.Release()
		return
	}

	if projectID == "" {
		client := wsentity.NewClient(c, projectID, user)
		_ = client.WriteJSON(client.Factory().Error(domainErrors.CodeValidation, "project ID is required"))
		client.Release()
		return
	}
	if !validation.IsValidSafeID(projectID) {
		client := wsentity.NewClient(c, projectID, user)
		_ = client.WriteJSON(client.Factory().Error(domainErrors.CodeValidation, "invalid project ID"))
		client.Release()
		return
	}
	if ticketProj, _ := c.Locals("ticketProjectId").(string); ticketProj != "" && ticketProj != projectID {
		client := wsentity.NewClient(c, projectID, user)
		_ = client.WriteJSON(client.Factory().Error(domainErrors.CodeAccessDenied, "ticket is not valid for this project"))
		client.Release()
		return
	}

	h.logger.Infof("New WebSocket connection: projectID=%s, userID=%s", projectID, user.ID)
	client := wsentity.NewClient(c, projectID, user)

	if h.access != nil {
		read, write, err := h.access.Access(context.Background(), projectID, user.ID)
		if err != nil {
			h.logger.Errorf("access check failed for user=%s project=%s: %v", user.ID, projectID, err)
			_ = client.WriteJSON(client.Factory().Error(domainErrors.CodeInternal, "could not check access"))
			client.Release()
			return
		}
		if !read {
			// Same reply whether the project is missing or private.
			_ = client.WriteJSON(client.Factory().Error(domainErrors.CodeAccessDenied, "you do not have access to this project"))
			client.Release()
			return
		}
		client.ReadOnly = !write
	}

	c.SetReadLimit(maxWSMessageBytes)
	_ = c.SetReadDeadline(time.Now().Add(wsPongWait))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	ck := connKey(projectID, user.ID)
	h.activeConnsMu.Lock()
	if old, exists := h.activeConns[ck]; exists {
		h.logger.Infof("Closing stale connection for user=%s project=%s", user.ID, projectID)
		_ = h.hub.Unregister(h.ctx, old)
		old.Close()
	}
	h.activeConns[ck] = client
	h.activeConnsMu.Unlock()

	pingStop := make(chan struct{})
	var pingWG sync.WaitGroup
	pingWG.Add(1)
	go func() {
		defer pingWG.Done()
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-pingStop:
				return
			case <-h.ctx.Done():
				return
			case <-ticker.C:
				if err := client.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
					return
				}
			}
		}
	}()

	ctx := context.Background()

	defer func() {
		close(pingStop)
		pingWG.Wait()

		_ = h.hub.Unregister(h.ctx, client)

		h.activeConnsMu.Lock()
		if h.activeConns[ck] == client {
			delete(h.activeConns, ck)
		}
		h.activeConnsMu.Unlock()

		// Must be last: fences off the writer goroutine before the library recycles the conn.
		client.Release()
	}()

	h.readWSLoop(ctx, c, client)
}

func (h *Handler) readWSLoop(ctx context.Context, c *websocket.Conn, client *wsentity.Client) {
	limiter := newTokenBucket(wsRateCapacity, wsRateRefillPerSec)

	for {
		var msg domainEntity.Message
		if err := c.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Errorf("websocket error: %v", err)
			}
			break
		}
		_ = c.SetReadDeadline(time.Now().Add(wsPongWait))

		if !limiter.allow() {
			h.logger.Errorf("rate limit exceeded for userID=%s projectID=%s", client.User.ID, client.ProjectID)
			errMsg := client.Factory().ErrorForOp(domainErrors.CodeValidation, "rate limit exceeded", msg.ClientOpID)
			errMsg.RequestID = msg.RequestID
			if sendErr := client.Send(errMsg); sendErr != nil {
				h.logger.Errorf("failed to send rate limit error to client: %v", sendErr)
			}
			continue
		}

		h.logger.Infof("Received message: type=%s from userID=%s", msg.Type, client.User.ID)
		h.logger.Debugf("Message details: %+v", msg)

		if err := h.processMessage(ctx, client, msg); err != nil {
			h.logger.Errorf("Error processing message %s: %v", msg.Type, err)

			// Never expose raw internal errors to WebSocket clients.
			errCode := domainErrors.CodeInternal
			errText := "internal error"
			var appErr *domainErrors.AppError
			switch {
			case errors.As(err, &appErr):
				errCode = appErr.Code
				errText = appErr.Message
			case errors.Is(err, domainErrors.ErrNotJoined):
				errCode = domainErrors.CodeJoinRequired
				errText = domainErrors.ErrNotJoined.Error()
			}

			errMsg := client.Factory().ErrorForOp(errCode, errText, msg.ClientOpID)
			errMsg.RequestID = msg.RequestID
			if sendErr := client.Send(errMsg); sendErr != nil {
				h.logger.Errorf("failed to send error to client: %v", sendErr)
			}
		}
	}
}

type tokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64
	last       time.Time
}

func newTokenBucket(capacity, refillRate float64) *tokenBucket {
	return &tokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		last:       time.Now(),
	}
}

func (b *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.refillRate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// processMessage resolves the handler for a message and executes it.
func (h *Handler) processMessage(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	// Ensure consistency in the incoming message metadata
	msg.ProjectID = client.ProjectID
	if msg.PageID == "" {
		msg.PageID = client.GetPageID()
	}

	if client.ReadOnly && mutating[msg.Type] {
		return domainErrors.New(domainErrors.CodeAccessDenied, "you have read-only access to this project", nil)
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
