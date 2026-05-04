package main

import (
	"testing"
)

func newTestClient(tenantID string, bufSize int) *Client {
	return &Client{
		tenantID: tenantID,
		send:     make(chan []byte, bufSize),
	}
}

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	c := newTestClient("t1", 4)
	c.hub = hub

	hub.Register("t1", c)
	hub.mu.RLock()
	if _, ok := hub.clients["t1"][c]; !ok {
		t.Error("client should be registered under t1")
	}
	hub.mu.RUnlock()

	hub.Unregister("t1", c)
	hub.mu.RLock()
	_, tenantExists := hub.clients["t1"]
	hub.mu.RUnlock()
	if tenantExists {
		t.Error("tenant map entry should be removed after last client unregisters")
	}
}

func TestHub_UnregisterClosesChannel(t *testing.T) {
	hub := NewHub()
	c := newTestClient("t2", 4)
	c.hub = hub

	hub.Register("t2", c)
	hub.Unregister("t2", c)

	// Receiving from a closed channel returns immediately with zero value.
	_, open := <-c.send
	if open {
		t.Error("send channel should be closed after Unregister")
	}
}

func TestHub_UnregisterIdempotent(t *testing.T) {
	hub := NewHub()
	c := newTestClient("t3", 4)
	c.hub = hub

	hub.Register("t3", c)
	hub.Unregister("t3", c)
	// Second Unregister on an already-removed client must not panic.
	hub.Unregister("t3", c)
}

func TestHub_BroadcastDelivers(t *testing.T) {
	hub := NewHub()
	c := newTestClient("ta", 4)
	hub.Register("ta", c)
	defer hub.Unregister("ta", c)

	msg := []byte(`{"type":"new_notification"}`)
	hub.Broadcast("ta", msg)

	select {
	case got := <-c.send:
		if string(got) != string(msg) {
			t.Errorf("got %q, want %q", got, msg)
		}
	default:
		t.Fatal("expected message in send channel, got nothing")
	}
}

func TestHub_BroadcastTenantIsolation(t *testing.T) {
	hub := NewHub()
	cA := newTestClient("tenant-A", 4)
	cB := newTestClient("tenant-B", 4)
	hub.Register("tenant-A", cA)
	hub.Register("tenant-B", cB)
	defer hub.Unregister("tenant-A", cA)
	defer hub.Unregister("tenant-B", cB)

	hub.Broadcast("tenant-A", []byte(`{"for":"A"}`))

	select {
	case <-cA.send:
		// expected
	default:
		t.Error("tenant-A client should have received the broadcast")
	}

	select {
	case msg := <-cB.send:
		t.Errorf("tenant-B should not receive tenant-A broadcast, got %q", msg)
	default:
		// expected
	}
}

func TestHub_BroadcastDropsWhenFull(t *testing.T) {
	hub := NewHub()
	// Zero-capacity channel is always full.
	c := newTestClient("t-full", 0)
	hub.Register("t-full", c)
	defer hub.Unregister("t-full", c)

	// Should not block or panic.
	hub.Broadcast("t-full", []byte(`{"overflow":true}`))
}

func TestHub_BroadcastUnknownTenant(t *testing.T) {
	hub := NewHub()
	// Should not panic when broadcasting to a tenant with no clients.
	hub.Broadcast("no-such-tenant", []byte(`{}`))
}
