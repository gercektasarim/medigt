package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"
)

// silentLogger keeps test output clean.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startHub runs the hub in a goroutine and returns a cancel function.
func startHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	hub := NewHub(silentLogger())
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	return hub, cancel
}

// readPayload blocks for a payload from the client's send channel with a
// generous test timeout so a stuck dispatch shows up as a clear failure
// rather than a deadlock.
func readPayload(t *testing.T, c *Client) []byte {
	t.Helper()
	select {
	case p := <-c.send:
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for payload")
		return nil
	}
}

// expectNoPayload asserts the channel stays quiet for a short window —
// used when verifying that scope filtering correctly hides messages.
func expectNoPayload(t *testing.T, c *Client) {
	t.Helper()
	select {
	case p := <-c.send:
		t.Fatalf("expected no payload, got %s", p)
	case <-time.After(150 * time.Millisecond):
		// good
	}
}

func TestHub_Publish_DeliversToMatchingSubscribers(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := newClient("c1")
	c.subscribe(ScopeBranch, "branch-A")
	hub.registerCh <- c
	// Wait for registration to land in the hub's map.
	time.Sleep(20 * time.Millisecond)

	hub.Publish(Event{
		Type:    "lab_result:new",
		Scope:   ScopeBranch,
		ScopeID: "branch-A",
		Payload: map[string]any{"order_id": "x"},
	})

	payload := readPayload(t, c)
	var ev Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "lab_result:new" {
		t.Fatalf("got type %q, want lab_result:new", ev.Type)
	}
	if ev.TS == 0 {
		t.Fatal("expected TS to be populated by Publish")
	}
}

func TestHub_Publish_FiltersByScope(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	a := newClient("a")
	a.subscribe(ScopeBranch, "branch-A")
	b := newClient("b")
	b.subscribe(ScopeBranch, "branch-B")
	hub.registerCh <- a
	hub.registerCh <- b
	time.Sleep(20 * time.Millisecond)

	hub.Publish(Event{Type: "x", Scope: ScopeBranch, ScopeID: "branch-A"})

	readPayload(t, a)      // a is subscribed → must receive
	expectNoPayload(t, b)  // b is NOT subscribed → must stay silent
}

func TestHub_Publish_FiltersByScopeKind(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := newClient("c")
	c.subscribe(ScopeBranch, "X")
	hub.registerCh <- c
	time.Sleep(20 * time.Millisecond)

	// Same id, different scope kind — must not deliver.
	hub.Publish(Event{Type: "x", Scope: ScopeOrganization, ScopeID: "X"})
	expectNoPayload(t, c)
}

func TestHub_Publish_MultipleSubscriptionsPerClient(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := newClient("c")
	c.subscribe(ScopeBranch, "B1")
	c.subscribe(ScopePatient, "P9")
	hub.registerCh <- c
	time.Sleep(20 * time.Millisecond)

	hub.Publish(Event{Type: "appointment", Scope: ScopeBranch, ScopeID: "B1"})
	readPayload(t, c)

	hub.Publish(Event{Type: "vitals", Scope: ScopePatient, ScopeID: "P9"})
	readPayload(t, c)
}

func TestHub_Remove_StopsDelivery(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := newClient("c")
	c.subscribe(ScopeBranch, "B")
	hub.registerCh <- c
	time.Sleep(20 * time.Millisecond)

	hub.Publish(Event{Type: "x", Scope: ScopeBranch, ScopeID: "B"})
	readPayload(t, c)

	hub.removeCh <- c.id
	time.Sleep(20 * time.Millisecond)

	// Hub closes the send channel on remove — the goroutine driving the
	// WebSocket writer treats a closed channel as a normal shutdown.
	if _, open := <-c.send; open {
		t.Fatal("send channel should be closed after remove")
	}

	// And a fresh publish must NOT panic on a removed client. Use a
	// different topic so a stale matcher would fail more loudly.
	hub.Publish(Event{Type: "y", Scope: ScopeBranch, ScopeID: "B"})
	time.Sleep(20 * time.Millisecond) // give the hub a moment; no panic = pass
}

func TestHub_FullSendBuffer_DropsRatherThanBlocks(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	// Create a client whose buffer we'll never drain — the hub must not
	// block on us; subsequent events for this client get dropped and the
	// hub keeps serving other clients.
	c := newClient("stuck")
	c.subscribe(ScopeBranch, "B")
	hub.registerCh <- c
	time.Sleep(20 * time.Millisecond)

	// Buffer is 64 — flood with more events than that. The hub log warns
	// and drops; nothing should hang.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			hub.Publish(Event{Type: "x", Scope: ScopeBranch, ScopeID: "B"})
		}
		close(done)
	}()
	select {
	case <-done:
		// good — Publish never blocked.
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked when client buffer was full")
	}
}

func TestHub_Publish_StampsTSWhenZero(t *testing.T) {
	hub, cancel := startHub(t)
	defer cancel()

	c := newClient("c")
	c.subscribe(ScopeUser, "u")
	hub.registerCh <- c
	time.Sleep(20 * time.Millisecond)

	hub.Publish(Event{Type: "x", Scope: ScopeUser, ScopeID: "u"}) // TS=0
	payload := readPayload(t, c)
	var ev Event
	_ = json.Unmarshal(payload, &ev)
	if ev.TS == 0 {
		t.Fatal("Publish should auto-stamp TS")
	}
}
