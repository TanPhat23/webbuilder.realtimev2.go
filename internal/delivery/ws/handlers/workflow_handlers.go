package handlers

import (
	"context"
	"encoding/json"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
)

func (h *Handler) handleWorkflowCreate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Workflow.CreateWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().WorkflowCreate(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleWorkflowUpdate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Workflow.UpdateWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().WorkflowUpdate(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleWorkflowDelete(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadWorkflowDelete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Workflow.RemoveWorkflow(ctx, client.ProjectID, payload.ID); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().WorkflowDelete(payload.ID)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementWorkflowCreate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadElementWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.ElementWorkflows.CreateElementWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().ElementWorkflowCreate(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementWorkflowUpdate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadElementWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.ElementWorkflows.UpdateElementWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().ElementWorkflowUpdate(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementWorkflowDelete(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadElementWorkflowDelete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.ElementWorkflows.RemoveElementWorkflow(ctx, client.ProjectID, payload.ElementID); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().ElementWorkflowDelete(payload.ElementID)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}