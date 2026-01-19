# WebSocket API Reference

This document provides a detailed reference for the WebSocket API used in the Web Builder real-time synchronization system.

## 1. Connection

The WebSocket endpoint is dynamic based on the project ID.

- **URL:** `ws://<host>:<port>/ws/:project`
- **Protocol:** `json`

Example: `ws://localhost:3000/ws/my-project-id`

---

## 2. Message Envelope

All communication (incoming and outgoing) uses a standard envelope:

```json
{
  "type": "string",
  "projectId": "string",
  "pageId": "string",
  "userId": "string",
  "timestamp": 123456789,
  "payload": {}
}
```

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
*Note: Upon joining, the server will automatically respond with a `sync` message containing the current page state.*

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
- **Payload:**
```json
{
  "element": {
    "type": "text",
    "properties": {},
    "styles": {},
    "order": 0
  }
}
```

### Update Element (Partial)
Updates only the provided fields.
- **Type:** `element:update`
- **Payload:**
```json
{
  "id": "uuid-string",
  "properties": { "content": "Hello World" },
  "styles": { "color": "red" },
  "settings": {}
}
```

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
Sent automatically after a successful `join` or requested via `sync`.
- **Type:** `sync`
- **Payload:**
```json
{
  "elements": [...],
  "users": [...]
}
```

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

| Code | Description |
| :--- | :--- |
| `VALIDATION_ERROR` | The payload data failed schema validation. |
| `NOT_FOUND` | The requested element or project does not exist. |
| `JOIN_REQUIRED` | You attempted an operation before sending a `join` message. |
| `PROCESS_ERROR` | An internal error occurred while processing the request. |
