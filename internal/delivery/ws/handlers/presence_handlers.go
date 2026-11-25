package handlers

import (
	"context"
	"encoding/json"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	"webbuilder/internal/utils/validation"
)

const (
	maxPresenceMetaKeys  = 16
	maxPresenceMetaBytes = 4096
)

func (h *Handler) joinPage(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if err := h.authorizeJoin(ctx, client, msg.PageID); err != nil {
		return err
	}

	oldPage := client.PageID()
	if oldPage != "" && oldPage != msg.PageID {
		_ = h.hub.Unregister(ctx, client)
	}
	client.SetPageID(msg.PageID)
	if err := h.hub.Register(ctx, client); err != nil {
		return err
	}

	if err := h.sendPageSync(ctx, client, msg.RequestID); err != nil {
		return err
	}
	if err := h.sendLogicSync(ctx, client, msg.RequestID); err != nil {
		h.logger.Errorf("logic sync on join failed: %v", err)
	}
	h.sendProjectSync(ctx, client, msg.RequestID)
	return nil
}

func (h *Handler) authorizeJoin(ctx context.Context, client *wsentity.Client, pageID string) error {
	if pageID == "" {
		return domainErrors.New(domainErrors.CodeValidation, "pageId is required to join", nil)
	}
	if !validation.IsSafeID(pageID) {
		return domainErrors.New(domainErrors.CodeValidation, "invalid pageId", nil)
	}
	if h.pageChecker == nil {
		return nil
	}
	belongs, err := h.pageChecker.PageBelongsToProject(ctx, client.ProjectID, pageID)
	if err != nil {
		h.logger.Errorf("page ownership check failed for project=%s page=%s: %v", client.ProjectID, pageID, err)
		return domainErrors.New(domainErrors.CodeInternal, "could not verify page", nil)
	}
	if !belongs {
		return domainErrors.New(domainErrors.CodeNotFound, "page not found in project", nil)
	}
	return nil
}

func (h *Handler) sendPageSync(ctx context.Context, client *wsentity.Client, requestID string) error {
	pageID := client.PageID()
	elements, seq, err := h.uc.Element.SyncPageWithSeq(ctx, client.ProjectID, pageID)
	if err != nil {
		return err
	}
	elementWorkflows, _ := h.uc.ElementWorkflows.ListElementWorkflowsByPage(ctx, client.ProjectID, pageID)
	users := h.hub.RoomParticipants(client.RoomID())
	syncMsg, _ := client.NewMessageFactory().NewPageSyncAtMessage(elements, elementWorkflows, users, seq)
	syncMsg.RequestID = requestID
	_ = client.Send(syncMsg)
	return nil
}

// sendProjectSync sends project state only to the joining client, not to every room.
// Broadcasting sync:project on every join creates a message storm during rapid
// reconnects; other clients already have the project state and receive granular
// updates via element/workflow operation broadcasts.
func (h *Handler) sendProjectSync(ctx context.Context, client *wsentity.Client, requestID string) {
	elements, _ := h.uc.Element.ListElementsByProject(ctx, client.ProjectID)
	workflows, _ := h.uc.Workflow.ListWorkflowsByProject(ctx, client.ProjectID)
	elementWorkflows, _ := h.uc.ElementWorkflows.ListElementWorkflowsByProject(ctx, client.ProjectID)
	syncMsg, _ := client.NewMessageFactory().NewProjectSyncMessage(elements, workflows, elementWorkflows, nil)
	syncMsg.RequestID = requestID
	_ = client.Send(syncMsg)
}

func (h *Handler) leavePage(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID() != "" {
		_ = h.hub.Unregister(ctx, client)
		client.SetPageID("")
	}
	return nil
}

func (h *Handler) updatePresence(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID() == "" {
		return domainErrors.ErrNotJoined
	}
	if client.User == nil || client.User.ID == "" {
		return domainErrors.New(domainErrors.CodeUnauthorized, "unauthorized", nil)
	}

	var payload domainEntity.PayloadPresence
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}
	if err := validatePresence(payload); err != nil {
		return err
	}

	// Enforce server-authenticated identity so clients cannot spoof other users.
	payload.UserID = client.User.ID
	payload.UserName = client.DisplayName()

	// Store the client's latest presence locally so sync responses reflect it,
	// then broadcast the presence to other clients.
	client.SetPresence(payload)

	broadcastMsg, _ := client.NewMessageFactory().NewPresenceMessage(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func validatePresence(payload domainEntity.PayloadPresence) error {
	if len(payload.ElementID) > 128 {
		return domainErrors.New(domainErrors.CodeValidation, "presence elementId is too long", nil)
	}
	if len(payload.Meta) > maxPresenceMetaKeys {
		return domainErrors.New(domainErrors.CodeValidation, "presence meta exceeds maximum key count", nil)
	}
	if len(payload.Meta) > 0 {
		rawMeta, err := json.Marshal(payload.Meta)
		if err != nil || len(rawMeta) > maxPresenceMetaBytes {
			return domainErrors.New(domainErrors.CodeValidation, "presence meta exceeds maximum size", nil)
		}
	}
	if a := payload.Agent; a != nil {
		if len(a.Name) > 128 || len(a.Status) > 128 || len(a.Action) > 256 ||
			(a.ElementID != nil && len(*a.ElementID) > 128) {
			return domainErrors.New(domainErrors.CodeValidation, "presence agent fields exceed maximum length", nil)
		}
	}
	return nil
}

func (h *Handler) syncPage(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	pageID := client.PageID()
	if pageID == "" {
		return domainErrors.ErrNotJoined
	}

	elements, seq, err := h.uc.Element.SyncPageWithSeq(ctx, client.ProjectID, pageID)
	if err != nil {
		return err
	}

	elementWorkflows, _ := h.uc.ElementWorkflows.ListElementWorkflowsByPage(ctx, client.ProjectID, pageID)

	users := h.hub.RoomParticipants(client.RoomID())
	syncMsg, _ := client.NewMessageFactory().NewPageSyncAtMessage(elements, elementWorkflows, users, seq)
	syncMsg.RequestID = msg.RequestID
	_ = client.Send(syncMsg)
	return nil
}

func (h *Handler) syncProject(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	elements, err := h.uc.Element.ListElementsByProject(ctx, client.ProjectID)
	if err != nil {
		return err
	}

	workflows, _ := h.uc.Workflow.ListWorkflowsByProject(ctx, client.ProjectID)
	elementWorkflows, _ := h.uc.ElementWorkflows.ListElementWorkflowsByProject(ctx, client.ProjectID)

	syncMsg, err := client.NewMessageFactory().NewProjectSyncMessage(elements, workflows, elementWorkflows, nil)
	if err != nil {
		return err
	}

	syncMsg.RequestID = msg.RequestID
	return client.Send(syncMsg)
}
