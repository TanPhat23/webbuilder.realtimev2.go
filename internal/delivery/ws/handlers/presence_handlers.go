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

func (h *Handler) handleJoin(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if msg.PageID == "" {
		return domainErrors.New(domainErrors.CodeValidation, "pageId is required to join", nil)
	}
	if !validation.IsValidSafeID(msg.PageID) {
		return domainErrors.New(domainErrors.CodeValidation, "invalid pageId", nil)
	}

	if h.pageChecker != nil {
		belongs, err := h.pageChecker.PageBelongsToProject(ctx, client.ProjectID, msg.PageID)
		if err != nil {
			h.logger.Errorf("page ownership check failed for project=%s page=%s: %v", client.ProjectID, msg.PageID, err)
			return domainErrors.New(domainErrors.CodeInternal, "could not verify page", nil)
		}
		if !belongs {
			return domainErrors.New(domainErrors.CodeNotFound, "page not found in project", nil)
		}
	}

	oldPage := client.GetPageID()
	if oldPage != "" && oldPage != msg.PageID {
		_ = h.hub.Unregister(ctx, client)
	}

	client.SetPageID(msg.PageID)
	if err := h.hub.Register(ctx, client); err != nil {
		return err
	}

	pageID := client.GetPageID()
	elements, seq, err := h.uc.Element.SyncPageWithSeq(ctx, client.ProjectID, pageID)
	if err != nil {
		return err
	}
	elementWorkflows, _ := h.uc.ElementWorkflows.GetWorkflowsByPage(ctx, client.ProjectID, pageID)

	users := h.hub.GetRoomParticipants(client.RoomID())
	syncMsg, _ := client.Factory().SyncPageAt(elements, elementWorkflows, users, seq)
	syncMsg.RequestID = msg.RequestID
	_ = client.Send(syncMsg)

	if err := h.sendLogicSync(ctx, client, msg.RequestID); err != nil {
		h.logger.Errorf("logic sync on join failed: %v", err)
	}

	projElements, _ := h.uc.Element.GetElementsByProjectID(ctx, client.ProjectID)
	projWorkflows, _ := h.uc.Workflow.GetWorkflowsByProject(ctx, client.ProjectID)
	projElementWorkflows, _ := h.uc.ElementWorkflows.GetWorkflowsByProject(ctx, client.ProjectID)

	// Send project sync only to the joining client — NOT broadcast to all rooms.
	// Broadcasting sync:project to every room on every join creates a message storm
	// during rapid reconnects (each reconnect triggers a full project sync to ALL
	// other clients across ALL pages). The joining client needs the project state;
	// other clients already have it and will receive granular updates via element/workflow
	// operation broadcasts.
	syncProjMsg, _ := client.Factory().SyncProject(projElements, projWorkflows, projElementWorkflows, nil)
	syncProjMsg.RequestID = msg.RequestID
	_ = client.Send(syncProjMsg)

	return nil
}

func (h *Handler) handleLeave(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.GetPageID() != "" {
		_ = h.hub.Unregister(ctx, client)
		client.SetPageID("")
	}
	return nil
}

func (h *Handler) handlePresence(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.GetPageID() == "" {
		return domainErrors.ErrNotJoined
	}
	if client.User == nil || client.User.ID == "" {
		return domainErrors.New(domainErrors.CodeUnauthorized, "unauthorized", nil)
	}

	var payload domainEntity.PayloadPresence
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

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
	if payload.Agent != nil {
		if len(payload.Agent.Name) > 128 || len(payload.Agent.Status) > 128 || len(payload.Agent.Action) > 256 ||
			(payload.Agent.ElementID != nil && len(*payload.Agent.ElementID) > 128) {
			return domainErrors.New(domainErrors.CodeValidation, "presence agent fields exceed maximum length", nil)
		}
	}

	// Enforce server-authenticated identity so clients cannot spoof other users.
	payload.UserID = client.User.ID
	payload.UserName = client.DisplayName()

	// Store the client's latest presence locally so sync responses reflect it,
	// then broadcast the presence to other clients.
	client.SetPresence(payload)

	broadcastMsg, _ := client.Factory().BroadcastPresence(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleSync(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	pageID := client.GetPageID()
	if pageID == "" {
		return domainErrors.ErrNotJoined
	}

	elements, seq, err := h.uc.Element.SyncPageWithSeq(ctx, client.ProjectID, pageID)
	if err != nil {
		return err
	}

	elementWorkflows, _ := h.uc.ElementWorkflows.GetWorkflowsByPage(ctx, client.ProjectID, pageID)

	users := h.hub.GetRoomParticipants(client.RoomID())
	syncMsg, _ := client.Factory().SyncPageAt(elements, elementWorkflows, users, seq)
	syncMsg.RequestID = msg.RequestID
	_ = client.Send(syncMsg)
	return nil
}

func (h *Handler) handleSyncProject(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	elements, err := h.uc.Element.GetElementsByProjectID(ctx, client.ProjectID)
	if err != nil {
		return err
	}

	workflows, _ := h.uc.Workflow.GetWorkflowsByProject(ctx, client.ProjectID)
	elementWorkflows, _ := h.uc.ElementWorkflows.GetWorkflowsByProject(ctx, client.ProjectID)

	syncMsg, err := client.Factory().SyncProject(elements, workflows, elementWorkflows, nil)
	if err != nil {
		return err
	}

	syncMsg.RequestID = msg.RequestID
	return client.Send(syncMsg)
}
