# WebSocket API Reference

This document describes the WebSocket protocol used by the Web Builder real-time engine for collaborative page editing, presence updates, and state synchronization.

It is aligned with the system architecture and README:

- The server exposes a project-scoped WebSocket endpoint.
- Clients must join a page context before performing document operations.
- The server is authoritative for persisted document state.
- Synchronization is optimized around granular element updates rather than full-document writes.
- Failed operations are returned as structured `error` messages.

## 1. Connection

The WebSocket endpoint is scoped by project.

- **URL:** `ws://<host>:<port>/ws/:projectId`
- **Protocol:** JSON messages over WebSocket

Example:

`ws://localhost:8080/ws/my-project-id`

### Authentication

Authentication is performed during the WebSocket upgrade request.

- Server-side or Node.js clients can send an `Authorization` header.
- Browser clients cannot set arbitrary headers in the native WebSocket constructor.
- For browser clients, use a secure query parameter or cookie-based authentication if the server is configured to read it during upgrade.

Node.js example:

```/dev/null/ws-node-example.js#L1-4
const WebSocket = require('ws');
const ws = new WebSocket('ws://localhost:8080/ws/my-project-id', {
  headers: { Authorization: 'Bearer <token>' },
});
```

## 2. Message Envelope

All client-to-server and server-to-client messages use the same JSON envelope.

| Field       | Type     | Required                       | Description                                                                           |
| :---------- | :------- | :----------------------------- | :------------------------------------------------------------------------------------ |
| `type`      | `string` | Yes                            | Message type such as `join`, `presence`, or `element:update`                          |
| `projectId` | `string` | Yes                            | Project identifier                                                                    |
| `pageId`    | `string` | Yes for page-scoped operations | Page identifier                                                                       |
| `userId`    | `string` | Recommended                    | Acting user identifier                                                                |
| `requestId` | `string` | Optional                       | Client-generated correlation ID for matching optimistic actions with server responses |
| `timestamp` | `int64`  | Yes                            | Unix timestamp in milliseconds                                                        |
| `payload`   | `object` | Yes                            | Message-specific data                                                                 |

Example:

```/dev/null/websocket-envelope.json#L1-16
{
  "type": "element:create",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "user-abc",
  "requestId": "req-1234",
  "timestamp": 1680000000000,
  "payload": {
    "element": {
      "type": "text",
      "settings": { "content": "Hello World" },
      "styles": { "color": "black" },
      "order": 0,
      "parentId": null
    }
  }
}
```

## 3. Message Types Overview

### Presence and Connection

- `join`: Enter a page context and trigger an initial sync
- `leave`: Exit a page context
- `presence`: Broadcast cursor position or current selection

### Document Operations

- `element:create`: Create a new element
- `element:update`: Apply a partial update to an existing element
- `element:move`: Reorder or reparent an element
- `element:delete`: Remove an element

### Synchronization and Errors

- `sync`: Request or receive the authoritative page snapshot
- `error`: Report validation or processing failures

## 4. Client to Server Messages

## 4.1 `join`

Clients should send `join` before any page-level collaborative operations.

### Purpose

- Associates the connection with a page
- Enables the server to send the current authoritative state
- Establishes the context for later element operations and presence updates

### Payload

```/dev/null/join-payload.json#L1-3
{
  "pageId": "page-456"
}
```

### Example

```/dev/null/join-message.json#L1-9
{
  "type": "join",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "user-abc",
  "timestamp": 1680000000000,
  "payload": {
    "pageId": "page-456"
  }
}
```

### Behavior

- A successful `join` typically results in a server `sync` message.
- If the page or project cannot be resolved, the server returns `error`.

## 4.2 `leave`

Clients can explicitly leave the current page context.

### Payload

`leave` typically does not require additional payload fields.

```/dev/null/leave-message.json#L1-7
{
  "type": "leave",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "user-abc",
  "timestamp": 1680000005000,
  "payload": {}
}
```

### Behavior

- The server removes the client from the active page context.
- Presence associated with the connection should be cleared.

## 4.3 `presence`

Presence messages broadcast ephemeral collaboration state such as cursor position or selected element.

### Payload

```/dev/null/presence-payload.json#L1-8
{
  "userId": "user-abc",
  "userName": "Alice",
  "cursorX": 0.5,
  "cursorY": 0.8,
  "elementId": "uuid-string",
  "meta": {}
}
```

### Notes

- Presence is transient and should not be treated as persisted document state.
- The server may rebroadcast presence updates to other participants on the same page.

## 4.4 `element:create`

Creates a new element in the current page.

### Client Example

```/dev/null/element-create-client.json#L1-17
{
  "type": "element:create",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "user-abc",
  "requestId": "req-1",
  "timestamp": 1680000000000,
  "payload": {
    "element": {
      "type": "text",
      "settings": { "content": "Hello World" },
      "styles": { "fontSize": 14, "color": "black" },
      "order": 0,
      "parentId": null
    }
  }
}
```

### Behavior

- The server validates the payload.
- The server persists the new element.
- The server broadcasts the authoritative created element, including server-assigned fields such as `id`, `createdAt`, and `createdBy`.

### Server Broadcast Example

```/dev/null/element-create-server.json#L1-19
{
  "type": "element:create",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "system",
  "requestId": "req-1",
  "timestamp": 1680000001000,
  "payload": {
    "element": {
      "id": "uuid-1111",
      "type": "text",
      "settings": { "content": "Hello World" },
      "styles": { "fontSize": 14, "color": "black" },
      "order": 0,
      "parentId": null,
      "createdAt": 1680000001000,
      "createdBy": "user-abc"
    }
  }
}
```

## 4.5 `element:update`

Applies a partial update to an existing element.

### Client Example

```/dev/null/element-update-client.json#L1-14
{
  "type": "element:update",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "user-abc",
  "requestId": "req-2",
  "timestamp": 1680000010000,
  "payload": {
    "id": "uuid-1111",
    "settings": { "content": "Hello Universe" },
    "styles": { "color": "blue" }
  }
}
```

### Behavior

- Only provided fields are updated.
- The server merges the patch into the authoritative stored element.
- Clients should treat the server broadcast as the source of truth.

### Notes

- This protocol is intentionally patch-oriented to minimize network overhead.
- Conflict handling is resolved server-side.

## 4.6 `element:move`

Moves an element by changing its order, parent, or both.

### Payload

```/dev/null/element-move-payload.json#L1-5
{
  "id": "uuid-string",
  "parentId": "uuid-string or null",
  "order": 5
}
```

### Behavior

- Use this message when reordering siblings or reparenting an element in the hierarchy.
- The server validates the requested destination and broadcasts the authoritative result.

## 4.7 `element:delete`

Deletes an element from the page.

### Payload

```/dev/null/element-delete-payload.json#L1-3
{
  "id": "uuid-string"
}
```

### Behavior

- The server removes the element if it exists and the request is valid.
- The resulting broadcast informs other participants to remove the element locally.

## 4.8 `sync`

Requests a fresh snapshot of the current page state.

### Client Request

```/dev/null/sync-request.json#L1-7
{
  "type": "sync",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "user-abc",
  "timestamp": 1680000020000,
  "payload": {}
}
```

### Behavior

- Clients can request `sync` after reconnecting or when local state is suspected to be stale.
- The response is a full authoritative page snapshot.

## 5. Server to Client Messages

## 5.1 `sync`

The `sync` message provides the current authoritative state for a page.

It is typically sent:

- after a successful `join`
- after an explicit client `sync` request
- after reconnect flows where the client needs a fresh snapshot

### Payload

```/dev/null/sync-response.json#L1-23
{
  "elements": [
    {
      "id": "uuid-1111",
      "type": "text",
      "settings": { "content": "Hello World" },
      "styles": { "fontSize": 14, "color": "black" },
      "order": 0,
      "parentId": null,
      "createdAt": 1680000001000,
      "createdBy": "user-abc"
    }
  ],
  "users": [
    {
      "userId": "user-abc",
      "userName": "Alice",
      "presence": {
        "cursorX": 0.5,
        "cursorY": 0.2,
        "elementId": null
      }
    }
  ]
}
```

### Notes

- `elements` represents the current page document snapshot.
- `users` represents active collaborators and their latest known presence state.
- Clients should replace or reconcile local state using this payload as authoritative.

## 5.2 Broadcasted document operations

When a user performs a successful document operation, the server broadcasts the corresponding message type to other participants in the same page context.

Common broadcast types:

- `element:create`
- `element:update`
- `element:move`
- `element:delete`
- `presence`

These broadcasts are the authoritative stream of collaborative changes.

## 5.3 `error`

The server sends `error` when a message fails validation, cannot be processed, or references missing resources.

### Payload

```/dev/null/error-payload.json#L1-4
{
  "code": "VALIDATION_ERROR",
  "message": "Detailed error description"
}
```

If available, the server may also include the original `requestId` in the outer envelope so the client can map the failure back to an optimistic action.

## 6. Standard Error Codes

| Code               | Description                                                        |
| :----------------- | :----------------------------------------------------------------- |
| `VALIDATION_ERROR` | The payload failed schema or business validation                   |
| `NOT_FOUND`        | The requested project, page, or element does not exist             |
| `JOIN_REQUIRED`    | The client attempted a page-scoped operation before sending `join` |
| `PROCESS_ERROR`    | An internal processing error occurred                              |

## 7. Client Integration Notes

## 7.1 Reconnection

Clients should implement reconnection using exponential backoff with jitter.

Recommended behavior:

- retry with a bounded backoff window
- re-establish the WebSocket connection
- re-send `join` for the active page
- request `sync` if necessary
- reconcile optimistic local state against authoritative server messages

## 7.2 Optimistic UI

For a responsive collaborative experience:

- generate a client-side `requestId` for mutating operations
- apply an optimistic local update if needed
- reconcile using the server broadcast
- prefer server-assigned IDs and timestamps over local assumptions

## 7.3 Source of Truth

The server is authoritative for:

- persisted element state
- ordering and parent relationships
- conflict resolution
- the current synchronized page snapshot

Clients should always converge toward the state represented by `sync` and broadcasted server messages.
