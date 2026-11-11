package handlers

import (
	"context"
	"encoding/json"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	"webbuilder/internal/utils/validation"
)

// applyAndBroadcast runs one document-mutating element operation for a page.
//
// The mutation and the broadcast run inside the page's actor, so on this instance ops of a page
// are applied, assigned their seq and handed to the room strictly in order. apply performs the
// atomic mutation and returns the page seq assigned to it; build creates the broadcast message.
//
// On success the message (stamped with seq and the sender's clientOpId) is broadcast to EVERY
// client in the room including the sender, who uses it as its ack. On failure nothing is
// broadcast and the error is returned (the read loop reports it to the sender with clientOpId).
func (h *Handler) applyAndBroadcast(
	ctx context.Context,
	client *wsentity.Client,
	msg domainEntity.Message,
	pageID string,
	apply func(ctx context.Context) (int64, error),
	build func(f *wsentity.MessageFactory) (domainEntity.Message, error),
) error {
	var opErr error
	key := client.ProjectID + ":" + pageID

	if err := h.pages.Do(ctx, key, func() {
		seq, err := apply(ctx)
		if err != nil {
			opErr = err
			return
		}

		out, err := build(client.Factory())
		if err != nil {
			// The change is already committed and has consumed its seq; log loudly because
			// clients will see a gap and resync.
			h.logger.Errorf("failed to build broadcast for committed op (page=%s seq=%d type=%s): %v", pageID, seq, msg.Type, err)
			opErr = domainErrors.Wrap(domainErrors.CodeInternal, err)
			return
		}
		out.Seq = seq
		out.ClientOpID = msg.ClientOpID
		out.RequestID = msg.RequestID
		h.hub.Broadcast(client.RoomID(), out, nil)
	}); err != nil {
		return err
	}
	return opErr
}

func (h *Handler) handleElementCreate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadElementCreate
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if client.PageID != "" {
		payload.Element.PageID = client.PageID
	}
	if err := validation.ValidateElementCreate(&payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	return h.applyAndBroadcast(ctx, client, msg, payload.Element.PageID,
		func(ctx context.Context) (int64, error) {
			return h.uc.Element.CreateElement(ctx, client.ProjectID, &payload.Element)
		},
		func(f *wsentity.MessageFactory) (domainEntity.Message, error) {
			return f.CreateElement(payload.Element)
		},
	)
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

	properties := updateProperties(payload)

	return h.applyAndBroadcast(ctx, client, msg, client.PageID,
		func(ctx context.Context) (int64, error) {
			return h.uc.Element.PatchElement(ctx, client.ProjectID, client.PageID, payload.ID, properties, payload.Styles, payload.Settings)
		},
		func(f *wsentity.MessageFactory) (domainEntity.Message, error) {
			return f.UpdateElement(payload)
		},
	)
}

// updateProperties merges the explicit `properties` map with the top-level field form
// (name, content, src, href, tailwindStyles) that the web client sends. Explicit
// `properties` entries win if both are present.
func updateProperties(p domainEntity.PayloadElementUpdate) map[string]any {
	props := make(map[string]any, len(p.Properties)+5)
	set := func(key string, v *string) {
		if v != nil {
			props[key] = *v
		}
	}
	set("name", p.Name)
	set("content", p.Content)
	set("src", p.Src)
	set("href", p.Href)
	set("tailwindStyles", p.TailwindStyles)
	for k, v := range p.Properties {
		props[k] = v
	}
	return props
}

func (h *Handler) handleElementDelete(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementDelete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	return h.applyAndBroadcast(ctx, client, msg, client.PageID,
		func(ctx context.Context) (int64, error) {
			return h.uc.Element.RemoveElement(ctx, client.ProjectID, client.PageID, payload.ID)
		},
		func(f *wsentity.MessageFactory) (domainEntity.Message, error) {
			return f.DeleteElement(payload.ID)
		},
	)
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

	return h.applyAndBroadcast(ctx, client, msg, client.PageID,
		func(ctx context.Context) (int64, error) {
			_, seq, err := h.uc.Element.MoveElement(ctx, client.ProjectID, client.PageID, payload.ID, parentID, payload.Order)
			return seq, err
		},
		func(f *wsentity.MessageFactory) (domainEntity.Message, error) {
			return f.MoveElement(payload)
		},
	)
}
