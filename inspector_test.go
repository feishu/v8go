package v8go_test

import (
	"sync"
	"testing"

	v8 "rogchap.com/v8go"
)

type inspectorTestMessage struct {
	callID int
	body   string
}

type inspectorTestChannel struct {
	mu            sync.Mutex
	responses     []inspectorTestMessage
	notifications []string
	flushes       int
}

func (c *inspectorTestChannel) SendResponse(callID int, message []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responses = append(c.responses, inspectorTestMessage{callID: callID, body: string(append([]byte(nil), message...))})
}

func (c *inspectorTestChannel) SendNotification(message []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = append(c.notifications, string(append([]byte(nil), message...)))
}

func (c *inspectorTestChannel) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushes++
}

func (c *inspectorTestChannel) responseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.responses)
}

func TestInspectorConnectAndDispatch(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	ins, err := v8.NewInspector(iso)
	if err != nil {
		t.Fatalf("new inspector: %v", err)
	}
	defer ins.Close()

	if err := ins.NotifyContextCreated(ctx, v8.InspectorOptions{
		ContextGroupID: 1,
		Name:           "test",
		Origin:         "test://inspector",
	}); err != nil {
		t.Fatalf("notify context created: %v", err)
	}

	ch := &inspectorTestChannel{}
	session, err := ins.Connect(ctx, ch, v8.InspectorOptions{ContextGroupID: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	before := ch.responseCount()
	if err := session.Dispatch([]byte(`{"id":1,"method":"Runtime.enable"}`)); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got := ch.responseCount(); got <= before {
		t.Fatalf("expected dispatch response, got %d responses before and %d after", before, got)
	}

	ctx.Close()
	if err := session.Dispatch([]byte(`{"id":2,"method":"Runtime.enable"}`)); err == nil {
		t.Fatal("expected dispatch to fail after context close")
	}
}

func TestInspectorSessionCloseFromGoroutine(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	ins, err := v8.NewInspector(iso)
	if err != nil {
		t.Fatalf("new inspector: %v", err)
	}
	defer ins.Close()

	if err := ins.NotifyContextCreated(ctx, v8.InspectorOptions{
		ContextGroupID: 1,
		Name:           "test",
		Origin:         "test://inspector",
	}); err != nil {
		t.Fatalf("notify context created: %v", err)
	}

	ch := &inspectorTestChannel{}
	session, err := ins.Connect(ctx, ch, v8.InspectorOptions{ContextGroupID: 1})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	errs := make(chan error, 1)
	go func() {
		errs <- session.Close()
	}()
	if err := <-errs; err != nil {
		t.Fatalf("close from goroutine: %v", err)
	}
}
