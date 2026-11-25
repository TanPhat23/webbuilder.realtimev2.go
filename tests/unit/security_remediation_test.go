package unit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"

	"webbuilder/internal/config"
	"webbuilder/internal/delivery/http/middleware"
	"webbuilder/internal/delivery/ws/handlers"
	"webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	domainRepo "webbuilder/internal/domain/repository"
	domainUsecase "webbuilder/internal/domain/usecase"
	"webbuilder/internal/usecase"
	"webbuilder/internal/utils/element"
	"webbuilder/internal/utils/validation"
)

func TestIsValidSafeID(t *testing.T) {
	valid := []string{
		"proj1",
		"p1",
		"page-1_a",
		"3fa85f64-5717-4562-b3fc-2c963f66afa6",
		strings.Repeat("a", 128),
	}
	for _, id := range valid {
		if !validation.IsValidSafeID(id) {
			t.Fatalf("expected %q to be a valid safe ID", id)
		}
	}

	invalid := []string{
		"",
		"proj:page",
		"../etc",
		"a/b",
		"a\\b",
		"a b",
		"a\nb",
		"a\x00b",
		strings.Repeat("a", 129),
	}
	for _, id := range invalid {
		if validation.IsValidSafeID(id) {
			t.Fatalf("expected %q to be rejected as unsafe ID", id)
		}
	}
}

func TestValidateElementTreeBounds(t *testing.T) {
	buildChain := func(depth int) entity.Element {
		cur := entity.Element{Type: "text", PageID: "page-1"}
		for i := 1; i < depth; i++ {
			cur = entity.Element{
				Type:     "container",
				PageID:   "page-1",
				Elements: []entity.Element{cur},
			}
		}
		return cur
	}

	okTree := buildChain(validation.MaxElementDepth)
	if err := validation.ValidateElementForAdd(&okTree); err != nil {
		t.Fatalf("depth %d should be allowed, got: %v", validation.MaxElementDepth, err)
	}

	tooDeep := buildChain(validation.MaxElementDepth + 1)
	if err := validation.ValidateElementForAdd(&tooDeep); err == nil {
		t.Fatalf("expected depth %d to be rejected", validation.MaxElementDepth+1)
	}

	wideOK := entity.Element{Type: "container", PageID: "page-1"}
	for i := 0; i < validation.MaxElementNodes-1; i++ {
		wideOK.Elements = append(wideOK.Elements, entity.Element{Type: "text", PageID: "page-1"})
	}
	if err := validation.ValidateElementForAdd(&wideOK); err != nil {
		t.Fatalf("%d nodes should be allowed, got: %v", validation.MaxElementNodes, err)
	}

	wideTooMany := wideOK
	wideTooMany.Elements = append(wideTooMany.Elements, entity.Element{Type: "text", PageID: "page-1"})
	if err := validation.ValidateElementForAdd(&wideTooMany); err == nil {
		t.Fatalf("expected %d nodes to be rejected", validation.MaxElementNodes+1)
	}
}

func TestMergeMap_DepthBounded(t *testing.T) {
	makeDeep := func(depth int, leafKey string, leafVal any) map[string]any {
		cur := map[string]any{leafKey: leafVal}
		for i := depth - 1; i >= 1; i-- {
			cur = map[string]any{fmt.Sprintf("l%d", i): cur}
		}
		return cur
	}

	patch := makeDeep(25, "leaf", "value")
	merged := element.MergeMap(nil, patch)

	// Walk down merged and verify depth never exceeds MaxMergeMapDepth.
	depth := 0
	cur := merged
	for len(cur) > 0 {
		depth++
		var next map[string]any
		for _, v := range cur {
			if m, ok := v.(map[string]any); ok {
				next = m
				break
			}
		}
		if next == nil {
			break
		}
		cur = next
	}
	if depth > element.MaxMergeMapDepth {
		t.Fatalf("expected MergeMap depth <= %d, got %d", element.MaxMergeMapDepth, depth)
	}
}

type fakePageChecker struct {
	allowed map[string]bool
}

func (f *fakePageChecker) PageBelongsToProject(_ context.Context, projectID, pageID string) (bool, error) {
	return f.allowed[projectID+":"+pageID], nil
}

type failingElementUC struct {
	domainUsecase.ElementUseCase
	syncErr error
}

func (f failingElementUC) SyncPageWithSeq(ctx context.Context, projectID, pageID string) ([]entity.Element, int64, error) {
	if f.syncErr != nil {
		return nil, 0, f.syncErr
	}
	return f.ElementUseCase.SyncPageWithSeq(ctx, projectID, pageID)
}

func startHardenedWSServer(
	t *testing.T,
	checker handlers.PageChecker,
	allowedOrigins []string,
	wrapElemUC func(domainUsecase.ElementUseCase) domainUsecase.ElementUseCase,
) (baseURL string, docs *memDocRepo) {
	t.Helper()
	docs = newMemDocRepo()
	repo := &domainRepo.Repository{
		DocumentRepo: docs,
		ElementRepo: &memElementRepo{elements: []entity.Element{
			{ID: testElemID, Type: "text", PageID: testPage, Styles: map[string]any{"color": "black"}},
		}},
	}
	docUC := usecase.NewDocumentUseCase(repo)
	var elemUC domainUsecase.ElementUseCase = stubElementUC{usecase.NewElementUseCase(repo, docUC, nil)}
	if wrapElemUC != nil {
		elemUC = wrapElemUC(elemUC)
	}
	uc := &domainUsecase.UseCase{
		Element:          elemUC,
		Document:         docUC,
		Workflow:         stubWorkflowUC{},
		ElementWorkflows: stubElementWorkflowsUC{},
	}

	h := handlers.NewHandler(uc, nil, &mockLogger{})
	if checker != nil {
		h.RequirePageCheck(checker)
	}
	if len(allowedOrigins) > 0 {
		h.SetAllowedOrigins(allowedOrigins)
	}
	t.Cleanup(h.Shutdown)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	var n int
	var mu sync.Mutex
	app.Use("/ws", func(c *fiber.Ctx) error {
		if c.Query("noUser") == "1" {
			return c.Next()
		}
		mu.Lock()
		n++
		id := fmt.Sprintf("user-%d", n)
		mu.Unlock()
		first := "Verified"
		last := fmt.Sprintf("User%d", n)
		c.Locals("user", &entity.User{
			ID:        id,
			Email:     id + "@example.com",
			FirstName: &first,
			LastName:  &last,
		})
		return c.Next()
	})
	h.SetupRoutes(app)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on loopback: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return "ws://" + ln.Addr().String() + "/ws/", docs
}

func TestWS_SafeID_PageOwnership_And_ElementCreateIsolation(t *testing.T) {
	checker := &fakePageChecker{allowed: map[string]bool{
		testProject + ":" + testPage: true,
	}}
	baseURL, docs := startHardenedWSServer(t, checker, nil, nil)

	// 1. Unsafe projectID (contains ':') must be rejected.
	connBadProj, _, err := websocket.DefaultDialer.Dial(baseURL+"proj:victim", nil)
	if err != nil {
		t.Fatalf("dial bad project: %v", err)
	}
	defer connBadProj.Close()
	_ = connBadProj.SetReadDeadline(time.Now().Add(2 * time.Second))
	var errMsg wsIn
	if err := connBadProj.ReadJSON(&errMsg); err != nil || errMsg.Type != "error" {
		t.Fatalf("expected validation error for unsafe projectID, got err=%v msg=%+v", err, errMsg)
	}

	// 2. Connect to valid project.
	conn, _, err := websocket.DefaultDialer.Dial(baseURL+testProject, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	c := &wsTestClient{t: t, conn: conn}

	// 3. element:create before joining must fail.
	c.conn.WriteJSON(map[string]any{
		"type":      "element:create",
		"pageId":    testPage,
		"timestamp": time.Now().UnixMilli(),
		"payload":   map[string]any{"element": map[string]any{"type": "text", "pageId": "victim-page"}},
	})
	if m, ok := c.next("error", 2*time.Second); !ok {
		t.Fatal("expected error for element:create before join")
	} else {
		var p struct{ Code string `json:"code"` }
		_ = json.Unmarshal(m.Payload, &p)
		if p.Code != domainErrors.CodeJoinRequired {
			t.Fatalf("expected JOIN_REQUIRED, got %s", p.Code)
		}
	}

	// 4. join with unsafe pageId must fail.
	c.conn.WriteJSON(map[string]any{"type": "join", "pageId": "page:victim", "timestamp": time.Now().UnixMilli()})
	if m, ok := c.next("error", 2*time.Second); !ok {
		t.Fatal("expected error for unsafe pageId")
	} else {
		var p struct{ Code string `json:"code"` }
		_ = json.Unmarshal(m.Payload, &p)
		if p.Code != domainErrors.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %s", p.Code)
		}
	}

	// 5. join with cross-tenant pageId must fail.
	c.conn.WriteJSON(map[string]any{"type": "join", "pageId": "other-tenant-page", "timestamp": time.Now().UnixMilli()})
	if m, ok := c.next("error", 2*time.Second); !ok {
		t.Fatal("expected error for cross-tenant pageId")
	} else {
		var p struct{ Code string `json:"code"` }
		_ = json.Unmarshal(m.Payload, &p)
		if p.Code != domainErrors.CodeNotFound {
			t.Fatalf("expected NOT_FOUND, got %s", p.Code)
		}
	}

	// 6. Valid join succeeds, and element:create ignores forged payload.element.pageId.
	c.send("join", map[string]any{"pageId": testPage}, "")
	if _, ok := c.next("sync:page", 2*time.Second); !ok {
		t.Fatal("expected sync:page after valid join")
	}

	c.send("element:create", map[string]any{
		"element": map[string]any{"type": "text", "pageId": "forged-other-page"},
	}, "create-1")
	created, ok := c.next("element:create", 2*time.Second)
	if !ok {
		t.Fatal("expected element:create broadcast")
	}
	var createdPayload struct {
		Element entity.Element `json:"element"`
	}
	_ = json.Unmarshal(created.Payload, &createdPayload)
	if createdPayload.Element.PageID != testPage {
		t.Fatalf("expected server to force pageId=%q, got %q", testPage, createdPayload.Element.PageID)
	}
	if snap := docs.snapshot(testProject, "forged-other-page"); len(snap) != 0 {
		t.Fatalf("forged page should have no elements, got %v", snap)
	}
}

func TestWS_HandlePresence_IdentityAndMetaBounds(t *testing.T) {
	baseURL, _ := startHardenedWSServer(t, nil, nil, nil)

	// Connect two clients: sender (user-1) and observer (user-2).
	senderConn, _, err := websocket.DefaultDialer.Dial(baseURL+testProject, nil)
	if err != nil {
		t.Fatalf("dial sender: %v", err)
	}
	defer senderConn.Close()
	sender := &wsTestClient{t: t, conn: senderConn}

	// Presence before joining must fail.
	sender.send("presence", map[string]any{"userId": "spoofed", "userName": "Evil"}, "")
	if _, ok := sender.next("error", 2*time.Second); !ok {
		t.Fatal("expected error for presence before join")
	}

	sender.send("join", map[string]any{"pageId": testPage}, "")
	if _, ok := sender.next("sync:page", 2*time.Second); !ok {
		t.Fatal("sender missing sync:page")
	}

	observerConn, _, err := websocket.DefaultDialer.Dial(baseURL+testProject, nil)
	if err != nil {
		t.Fatalf("dial observer: %v", err)
	}
	defer observerConn.Close()
	observer := &wsTestClient{t: t, conn: observerConn}
	observer.send("join", map[string]any{"pageId": testPage}, "")
	if _, ok := observer.next("sync:page", 2*time.Second); !ok {
		t.Fatal("observer missing sync:page")
	}

	// Send presence attempting to spoof userId and userName.
	sender.send("presence", map[string]any{
		"userId":   "spoofed-admin",
		"userName": "Spoofed Admin",
		"cursorX":  42,
		"cursorY":  84,
	}, "")

	got, ok := observer.next("presence", 2*time.Second)
	if !ok {
		t.Fatal("observer did not receive presence broadcast")
	}
	var pres entity.PayloadPresence
	if err := json.Unmarshal(got.Payload, &pres); err != nil {
		t.Fatalf("unmarshal presence: %v", err)
	}
	if pres.UserID != "user-1" {
		t.Fatalf("expected server-enforced userId=user-1, got %q", pres.UserID)
	}
	if pres.UserName != "Verified User1" {
		t.Fatalf("expected server-enforced userName='Verified User1', got %q", pres.UserName)
	}

	// Oversized meta (> 16 keys) must be rejected.
	tooManyMeta := make(map[string]any)
	for i := 0; i < 17; i++ {
		tooManyMeta[fmt.Sprintf("k%d", i)] = i
	}
	sender.send("presence", map[string]any{"cursorX": 1, "cursorY": 2, "meta": tooManyMeta}, "")
	if _, ok := sender.next("error", 2*time.Second); !ok {
		t.Fatal("expected validation error for oversized presence meta keys")
	}

	// Oversized meta value (> 4096 bytes) must be rejected.
	sender.send("presence", map[string]any{"cursorX": 1, "cursorY": 2, "meta": map[string]any{"big": strings.Repeat("x", 5000)}}, "")
	if _, ok := sender.next("error", 2*time.Second); !ok {
		t.Fatal("expected validation error for oversized presence meta bytes")
	}
}

func TestWS_MissingUser_SanitizedErrors_ReadLimit_And_RateLimit(t *testing.T) {
	// 1. Non-AppError sanitization check.
	baseURL, _ := startHardenedWSServer(t, nil, nil, func(uc domainUsecase.ElementUseCase) domainUsecase.ElementUseCase {
		return failingElementUC{
			ElementUseCase: uc,
			syncErr:        errors.New("pq: secret internal database error connection string"),
		}
	})

	// Missing user in Locals must be rejected cleanly without panic.
	noUserConn, _, err := websocket.DefaultDialer.Dial(baseURL+testProject+"?noUser=1", nil)
	if err != nil {
		t.Fatalf("dial noUser: %v", err)
	}
	defer noUserConn.Close()
	_ = noUserConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var unauthMsg wsIn
	if err := noUserConn.ReadJSON(&unauthMsg); err != nil || unauthMsg.Type != "error" {
		t.Fatalf("expected UNAUTHORIZED error for missing user, got err=%v msg=%+v", err, unauthMsg)
	}

	// Non-AppError from SyncPageWithSeq must be sanitized to "internal error".
	conn, _, err := websocket.DefaultDialer.Dial(baseURL+testProject, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	c := &wsTestClient{t: t, conn: conn}
	c.send("join", map[string]any{"pageId": testPage}, "")
	errMsg, ok := c.next("error", 2*time.Second)
	if !ok {
		t.Fatal("expected error reply for internal failure")
	}
	var errPayload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(errMsg.Payload, &errPayload)
	if errPayload.Message != "internal error" || strings.Contains(errPayload.Message, "secret") {
		t.Fatalf("expected sanitized 'internal error', got %+v", errPayload)
	}

	// 2. Oversized message (> 512 KB) triggers read limit and closes connection.
	baseNormal, _ := startHardenedWSServer(t, nil, nil, nil)
	bigConn, _, err := websocket.DefaultDialer.Dial(baseNormal+testProject, nil)
	if err != nil {
		t.Fatalf("dial big: %v", err)
	}
	defer bigConn.Close()
	oversized := map[string]any{
		"type":      "presence",
		"pageId":    testPage,
		"timestamp": time.Now().UnixMilli(),
		"payload":   map[string]any{"meta": map[string]any{"blob": strings.Repeat("A", 600*1024)}},
	}
	_ = bigConn.WriteJSON(oversized)
	_ = bigConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var tmp wsIn
	if err := bigConn.ReadJSON(&tmp); err == nil {
		t.Fatalf("expected connection to close on oversized message (>512KB), got %+v", tmp)
	}

	// 3. Rate limit triggers when bursting > 60 messages rapidly.
	rlConn, _, err := websocket.DefaultDialer.Dial(baseNormal+testProject, nil)
	if err != nil {
		t.Fatalf("dial rl: %v", err)
	}
	defer rlConn.Close()
	rlClient := &wsTestClient{t: t, conn: rlConn}
	for i := 0; i < 80; i++ {
		rlClient.send("leave", map[string]any{}, fmt.Sprintf("rl-%d", i))
	}
	rateLimited := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m, ok := rlClient.next("error", 300*time.Millisecond)
		if !ok {
			break
		}
		if strings.Contains(string(m.Payload), "rate limit exceeded") {
			rateLimited = true
			break
		}
	}
	if !rateLimited {
		t.Fatal("expected rate limit exceeded error during burst")
	}
}

func TestWS_OriginRestriction_CSWSH(t *testing.T) {
	baseURL, _ := startHardenedWSServer(t, nil, []string{"https://app.example.com"}, nil)

	// Cross-origin request must be rejected.
	evilHeaders := http.Header{"Origin": []string{"https://evil.example.com"}}
	_, resp, err := websocket.DefaultDialer.Dial(baseURL+testProject, evilHeaders)
	if err == nil {
		t.Fatal("expected cross-origin WebSocket upgrade to fail")
	}
	if resp != nil && resp.StatusCode == http.StatusSwitchingProtocols {
		t.Fatal("expected non-101 response for disallowed origin")
	}

	// Allowed origin succeeds.
	goodHeaders := http.Header{"Origin": []string{"https://app.example.com"}}
	conn, _, err := websocket.DefaultDialer.Dial(baseURL+testProject, goodHeaders)
	if err != nil {
		t.Fatalf("expected allowed origin to connect, got: %v", err)
	}
	_ = conn.Close()

	// Empty origin (non-browser client) succeeds.
	connEmpty, _, err := websocket.DefaultDialer.Dial(baseURL+testProject, nil)
	if err != nil {
		t.Fatalf("expected empty origin to connect, got: %v", err)
	}
	_ = connEmpty.Close()
}

func TestConfigAndAuthMiddleware_Security(t *testing.T) {
	// LoadConfig refuses empty ALLOWED_ORIGINS in production.
	t.Setenv("ENV", "production")
	t.Setenv("ALLOWED_ORIGINS", "")
	if _, err := config.LoadConfig(); err == nil {
		t.Fatal("expected LoadConfig to fail when ENV=production and ALLOWED_ORIGINS is empty")
	}

	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com")
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("expected valid production config to load, got: %v", err)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://app.example.com" {
		t.Fatalf("unexpected AllowedOrigins: %+v", cfg.AllowedOrigins)
	}

	// IsUnsafeDevMode rejects production, staging, and sk_live_ keys.
	for _, tc := range []struct {
		name string
		cfg  config.Config
	}{
		{"production", config.Config{Env: "production", DevMode: true}},
		{"prod", config.Config{Env: "prod", DevMode: true}},
		{"staging", config.Config{Env: "staging", DevMode: true}},
		{"live_key", config.Config{Env: "development", DevMode: true, ClerkSecretKey: "sk_live_12345"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.cfg.IsUnsafeDevMode() {
				t.Fatalf("expected %s to be flagged unsafe dev mode", tc.name)
			}
			app := fiber.New()
			app.Use(middleware.AuthMiddleware(&tc.cfg))
			app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })
			resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			if resp.StatusCode != fiber.StatusInternalServerError {
				t.Fatalf("expected 500 fail-closed for unsafe dev mode, got %d", resp.StatusCode)
			}
		})
	}

	// Non-dev AuthMiddleware rejects __session cookie and ?token= query param.
	prodCfg := &config.Config{Env: "production", DevMode: false, ClerkSecretKey: "sk_test_dummy"}
	app := fiber.New()
	app.Use(middleware.AuthMiddleware(prodCfg))
	app.Get("/ws/p1", func(c *fiber.Ctx) error { return c.SendString("ok") })

	reqCookie := httptest.NewRequest("GET", "/ws/p1", nil)
	reqCookie.AddCookie(&http.Cookie{Name: "__session", Value: "some-jwt"})
	respCookie, err := app.Test(reqCookie)
	if err != nil || respCookie.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected __session cookie to be rejected with 401, got err=%v status=%d", err, respCookie.StatusCode)
	}

	reqQuery := httptest.NewRequest("GET", "/ws/p1?token=some-jwt", nil)
	respQuery, err := app.Test(reqQuery)
	if err != nil || respQuery.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected ?token= query param to be rejected with 401, got err=%v status=%d", err, respQuery.StatusCode)
	}
}
