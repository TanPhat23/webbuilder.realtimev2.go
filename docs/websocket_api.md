# WebSocket API Reference

This document provides a detailed reference for the WebSocket API used in the Web Builder real-time synchronization system.

## 1. Connection

The WebSocket endpoint is dynamic based on the project ID.

- **URL:** `ws://<host>:<port>/ws/:project`
- **Protocol:** `json`

Example: `ws://localhost:8080/ws/my-project-id`

Authentication note:

- The server expects an Authorization header on the WebSocket upgrade request (for example: `Authorization: Bearer <token>`). Node clients (e.g., `ws`) can send headers on the handshake.
- Note: Browsers' native WebSocket API cannot set arbitrary request headers on the initial handshake. For browser-based clients, pass the token in a query parameter (for example: `wss://host/ws/my-project-id?token=<jwt>`) or use cookies and ensure the server reads the token from the query string or cookie during the upgrade.
- Example (Node `ws` client):

```js
const WebSocket = require('ws');
const ws = new WebSocket('ws://localhost:8080/ws/my-project-id', {
  headers: { Authorization: 'Bearer <token>' },
});
```

---

## 2. Message Envelope

All communication (incoming and outgoing) uses a standard envelope:

The envelope includes the following fields:

| Field       | Type     | Description                                                                              |
| :---------- | :------- | :--------------------------------------------------------------------------------------- |
| `type`      | `string` | The action type (e.g., `element:update`)                                                 |
| `projectId` | `string` | ID of the project                                                                        |
| `pageId`    | `string` | ID of the page                                                                           |
| `userId`    | `string` | ID of the acting user (optional but recommended)                                         |
| `requestId` | `string` | Optional client-generated ID to correlate request/response (useful for acknowledgements) |
| `timestamp` | `int64`  | Unix milliseconds                                                                        |
| `payload`   | `object` | Action-specific data                                                                     |

Example envelope (client -> server):

```json
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
      "order": 0
    }
  }
}
```

On success, the server will broadcast an `element:create` message to other clients including server-assigned fields (e.g., `id`, `createdAt`, `createdBy`) and may include the original `requestId` for correlation.

---

## 3. Client to Server Messages

### Join Page

Must be sent before performing any element operations.

- **Type:** `join`
- **Payload:**

```json
{
  "pageId": "string"
}
```

_Note: Upon joining, the server will automatically respond with a `sync` message containing the current page state._

### Presence (Cursor/Selection)

Broadcasts your current activity to other users.

- **Type:** `presence`
- **Payload:**

```json
{
  "userId": "string",
  "userName": "string",
  "cursorX": 0.5,
  "cursorY": 0.8,
  "elementId": "uuid-string (optional)",
  "meta": {}
}
```

### Create Element

- **Type:** `element:create`

- **Client Payload Example:**

```json
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

- **Server Broadcast Example (after persistence):**

```json
{
  "type": "element:create",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "system",
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

Notes:

- The server is authoritative — clients should reconcile optimistic client updates with server broadcasts using `requestId` and the assigned `id`.
- If creation fails validation, the server will return an `error` message referencing the original `requestId` when available.

### Update Element (Partial)

Updates only the provided fields (patch semantics).

- **Type:** `element:update`

- **Client Payload Example:**

```json
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

- **Server Broadcast Example (merged patch):**

```json
{
  "type": "element:update",
  "projectId": "project-123",
  "pageId": "page-456",
  "userId": "user-abc",
  "timestamp": 1680000010100,
  "payload": {
    "id": "uuid-1111",
    "settings": { "content": "Hello Universe" },
    "styles": { "color": "blue" }
  }
}
```

Notes:

- The server merges partial updates into the stored element; the broadcasted message is authoritative.
- Conflict resolution is handled server-side (e.g., timestamp-based LWW or optimistic locking); clients should apply server broadcasts as source of truth.

### Move Element

Reparents or reorders an element.

- **Type:** `element:move`
- **Payload:**

```json
{
  "id": "uuid-string",
  "parentId": "uuid-string or null",
  "order": 5
}
```

### Delete Element

- **Type:** `element:delete`
- **Payload:**

```json
{
  "id": "uuid-string"
}
```

### Sync Request

Force a refresh of the current page state.

- **Type:** `sync`
- **Payload:** Empty

---

## 4. Server to Client Messages

### Initial State Sync

Sent automatically after a successful `join` or requested via `sync`. This message contains the authoritative document snapshot for the current page and the presence information for active users.

- **Type:** `sync`
- **Payload:**

```json
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

Client Snippets (JavaScript):

```javascript
const ws = new WebSocket('ws://localhost:8080/ws/my-project-id');

ws.addEventListener('open', () => {
  // join the page (must be done before element ops)
  ws.send(
    JSON.stringify({
      type: 'join',
      projectId: 'my-project-id',
      pageId: 'page-1',
      userId: 'user-1',
      timestamp: Date.now(),
      payload: { pageId: 'page-1' },
    })
  );
});

ws.addEventListener('message', (ev) => {
  const msg = JSON.parse(ev.data);
  switch (msg.type) {
    case 'sync':
      // replace local document with msg.payload.elements
      break;
    case 'element:create':
      // insert element
      break;
    case 'element:update':
      // apply patch
      break;
    case 'error':
      // handle errors
      break;
  }
});
```

Reconnection guidance:

- Use exponential backoff with jitter (e.g., base 500ms, multiplier 1.5, max 30s).
- On reconnect, re-send `join` (with same `userId`) and request a `sync` if needed.
- Use `requestId` and message `timestamp`/server-assigned `id` to deduplicate and reconcile missed messages.
- Apply server-sent `sync` as authoritative snapshot; reconcile client optimistic updates by matching `requestId` or element IDs.

### Broadcasted Operations

When another user performs an action (create, update, delete, move), the server broadcasts the exact same message type and payload to all other participants in the room.

### Error Report

Sent when an operation fails validation or encounters a system error.

- **Type:** `error`
- **Payload:**

```json
{
  "code": "VALIDATION_ERROR | NOT_FOUND | JOIN_REQUIRED",
  "message": "Detailed error description"
}
```

---

## 5. Standard Error Codes

| Code               | Description                                                 |
| :----------------- | :---------------------------------------------------------- |
| `VALIDATION_ERROR` | The payload data failed schema validation.                  |
| `NOT_FOUND`        | The requested element or project does not exist.            |
| `JOIN_REQUIRED`    | You attempted an operation before sending a `join` message. |
| `PROCESS_ERROR`    | An internal error occurred while processing the request.    |
