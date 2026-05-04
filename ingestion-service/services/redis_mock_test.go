package services

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeRedisClient is an in-memory, non-networked fake implementing RedisClient.
type fakeRedisClient struct {
	mu sync.Mutex
	kv map[string]string
	h  map[string]map[string]string
}

func newFakeRedis() *fakeRedisClient {
	return &fakeRedisClient{kv: make(map[string]string), h: make(map[string]map[string]string)}
}

func (f *fakeRedisClient) Ping(ctx context.Context) error { return nil }
func (f *fakeRedisClient) Close() error                   { return nil }
func (f *fakeRedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kv[key] = value.(string)
	return nil
}
func (f *fakeRedisClient) Get(ctx context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.kv[key]
	if !ok {
		return "", ErrRedisNil
	}
	return v, nil
}
func (f *fakeRedisClient) HSet(ctx context.Context, key string, values map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.h[key]; !ok {
		f.h[key] = make(map[string]string)
	}
	for k, v := range values {
		f.h[key][k] = v.(string)
	}
	return nil
}
func (f *fakeRedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.h[key]
	if !ok {
		return map[string]string{}, nil
	}
	copy := make(map[string]string, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy, nil
}
func (f *fakeRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeRedisClient) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.kv[key]; ok {
		return false, nil
	}
	f.kv[key] = value.(string)
	return true, nil
}
func (f *fakeRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var count int64
	for _, k := range keys {
		if _, ok := f.kv[k]; ok {
			count++
		}
	}
	return count, nil
}

// ErrRedisNil sentinel to indicate missing key like go-redis's Nil.
var ErrRedisNil = &fakeRedisError{"redis: nil"}

type fakeRedisError struct{ s string }

func (e *fakeRedisError) Error() string { return e.s }

func TestRedisCache_MockBehavior(t *testing.T) {
	f := newFakeRedis()
	// create cache with fake client
	c := &RedisCache{client: f, pubKeyTTL: 30 * time.Minute, nonceTTL: 60 * time.Second}

	ctx := context.Background()

	// Test public key cache
	if err := c.CacheDevicePublicKey(ctx, 1, "pk-1"); err != nil {
		t.Fatalf("CacheDevicePublicKey failed: %v", err)
	}
	got, err := c.GetDevicePublicKey(ctx, 1)
	if err != nil {
		t.Fatalf("GetDevicePublicKey failed: %v", err)
	}
	if got != "pk-1" {
		t.Fatalf("unexpected public key: %s", got)
	}

	// Test device state map
	if err := c.CacheDeviceState(ctx, 2, true, "pk-2"); err != nil {
		t.Fatalf("CacheDeviceState failed: %v", err)
	}
	isActive, publicKey, found, err := c.GetDeviceState(ctx, 2)
	if err != nil {
		t.Fatalf("GetDeviceState failed: %v", err)
	}
	if !found || !isActive || publicKey != "pk-2" {
		t.Fatalf("unexpected device state: found=%v isActive=%v pk=%s", found, isActive, publicKey)
	}

	// Update active status
	if err := c.UpdateDeviceActiveStatus(ctx, 2, false); err != nil {
		t.Fatalf("UpdateDeviceActiveStatus failed: %v", err)
	}
	isActive2, _, _, _ := c.GetDeviceState(ctx, 2)
	if isActive2 {
		t.Fatalf("expected device to be inactive after update")
	}

	// Nonce reservation and existence
	ok, err := c.ReserveNonce(ctx, 3, "n1")
	if err != nil || !ok {
		t.Fatalf("ReserveNonce failed: ok=%v err=%v", ok, err)
	}
	exists, err := c.NonceExists(ctx, 3, "n1")
	if err != nil || !exists {
		t.Fatalf("NonceExists failed: exists=%v err=%v", exists, err)
	}
	// MarkNonceUsed should fail to set again
	ok2, err := c.MarkNonceUsed(ctx, 3, "n1")
	if err != nil {
		t.Fatalf("MarkNonceUsed error: %v", err)
	}
	if ok2 {
		t.Fatalf("MarkNonceUsed returned true for already existing nonce")
	}
}
