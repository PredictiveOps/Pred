package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient defines the subset of redis.Client methods used by RedisCache.
type RedisClient interface {
	Ping(ctx context.Context) error
	Close() error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	HSet(ctx context.Context, key string, values map[string]interface{}) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	Exists(ctx context.Context, keys ...string) (int64, error)
}

type RedisCache struct {
	client    RedisClient
	pubKeyTTL time.Duration
	nonceTTL  time.Duration
}

func NewRedisCache(addr, password string, db int, pubKeyTTL, nonceTTL time.Duration) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	// wrap real client with adapter implementing RedisClient
	real := &realRedisClient{c: client}
	return &RedisCache{
		client:    real,
		pubKeyTTL: pubKeyTTL,
		nonceTTL:  nonceTTL,
	}, nil
}

// realRedisClient adapts *redis.Client to the RedisClient interface.
type realRedisClient struct{ c *redis.Client }

func (r *realRedisClient) Ping(ctx context.Context) error {
	return r.c.Ping(ctx).Err()
}
func (r *realRedisClient) Close() error { return r.c.Close() }
func (r *realRedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.c.Set(ctx, key, value, ttl).Err()
}
func (r *realRedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.c.Get(ctx, key).Result()
}
func (r *realRedisClient) HSet(ctx context.Context, key string, values map[string]interface{}) error {
	return r.c.HSet(ctx, key, values).Err()
}
func (r *realRedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.c.HGetAll(ctx, key).Result()
}
func (r *realRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return r.c.Expire(ctx, key, expiration).Result()
}
func (r *realRedisClient) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return r.c.SetNX(ctx, key, value, ttl).Result()
}
func (r *realRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.c.Exists(ctx, keys...).Result()
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

func (r *RedisCache) CacheDevicePublicKey(ctx context.Context, deviceID uint, pemKey string) error {
	key := fmt.Sprintf("device_pubkey:%d", deviceID)
	return r.client.Set(ctx, key, pemKey, r.pubKeyTTL)
}

func (r *RedisCache) GetDevicePublicKey(ctx context.Context, deviceID uint) (string, error) {
	key := fmt.Sprintf("device_pubkey:%d", deviceID)
	return r.client.Get(ctx, key)
}

func (r *RedisCache) CacheDeviceState(ctx context.Context, deviceID uint, isActive bool, publicKey string) error {
	key := fmt.Sprintf("device_state:%d", deviceID)
	if err := r.client.HSet(ctx, key, map[string]interface{}{
		"is_active":  strconv.FormatBool(isActive),
		"public_key": publicKey,
	}); err != nil {
		return err
	}

	_, err := r.client.Expire(ctx, key, r.pubKeyTTL)
	return err
}

func (r *RedisCache) GetDeviceState(ctx context.Context, deviceID uint) (bool, string, bool, error) {
	key := fmt.Sprintf("device_state:%d", deviceID)
	values, err := r.client.HGetAll(ctx, key)
	if err != nil {
		return false, "", false, err
	}
	if len(values) == 0 {
		return false, "", false, nil
	}

	isActive, err := strconv.ParseBool(values["is_active"])
	if err != nil {
		return false, "", false, err
	}

	return isActive, values["public_key"], true, nil
}

func (r *RedisCache) UpdateDeviceActiveStatus(ctx context.Context, deviceID uint, isActive bool) error {
	key := fmt.Sprintf("device_state:%d", deviceID)
	if err := r.client.HSet(ctx, key, map[string]interface{}{"is_active": strconv.FormatBool(isActive)}); err != nil {
		return err
	}

	_, err := r.client.Expire(ctx, key, r.pubKeyTTL)
	return err
}

func (r *RedisCache) ReserveNonce(ctx context.Context, deviceID uint, nonce string) (bool, error) {
	key := fmt.Sprintf("nonce:%d:%s", deviceID, nonce)
	return r.client.SetNX(ctx, key, "true", r.nonceTTL)
}

func (r *RedisCache) NonceExists(ctx context.Context, deviceID uint, nonce string) (bool, error) {
	key := fmt.Sprintf("nonce:%d:%s", deviceID, nonce)
	count, err := r.client.Exists(ctx, key)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *RedisCache) MarkNonceUsed(ctx context.Context, deviceID uint, nonce string) (bool, error) {
	key := fmt.Sprintf("nonce:%d:%s", deviceID, nonce)
	return r.client.SetNX(ctx, key, "true", r.nonceTTL)
}
