package handlers

import (
	"context"
	"encoding/json"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
)

func (h *Handler) createWorkflow(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Workflow.CreateWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.NewMessageFactory().NewWorkflowCreatedMessage(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) updateWorkflow(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Workflow.UpdateWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.NewMessageFactory().NewWorkflowUpdatedMessage(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) deleteWorkflow(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadWorkflowDelete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Workflow.DeleteWorkflow(ctx, client.ProjectID, payload.ID); err != nil {
		return err
	}

	broadcastMsg, _ := client.NewMessageFactory().NewWorkflowDeletedMessage(payload.ID)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) createElementWorkflow(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadElementWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.ElementWorkflows.CreateElementWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.NewMessageFactory().NewElementWorkflowCreatedMessage(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) updateElementWorkflow(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadElementWorkflow
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.ElementWorkflows.UpdateElementWorkflow(ctx, client.ProjectID, &payload.Workflow); err != nil {
		return err
	}

	broadcastMsg, _ := client.NewMessageFactory().NewElementWorkflowUpdatedMessage(payload.Workflow)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) deleteElementWorkflow(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadElementWorkflowDelete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.ElementWorkflows.DeleteElementWorkflow(ctx, client.ProjectID, payload.ElementID); err != nil {
		return err
	}

	broadcastMsg, _ := client.NewMessageFactory().NewElementWorkflowDeletedMessage(payload.ElementID)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}
