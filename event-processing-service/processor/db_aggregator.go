package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"event-processing-service/db"
)

type aggregationKey struct {
	tenantID string
	deviceID uint
}

// DBAggregator polls persisted events, aggregates unprocessed readings by
// tenant and device, publishes ML features, and marks rows processed.
type DBAggregator struct {
	gdb       *gorm.DB
	sink      FeatureSink
	interval  time.Duration
	batchSize int
}

func NewDBAggregator(gdb *gorm.DB, sink FeatureSink, interval time.Duration, batchSize int) *DBAggregator {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 500
	}

	return &DBAggregator{
		gdb:       gdb,
		sink:      sink,
		interval:  interval,
		batchSize: batchSize,
	}
}

// Run starts the polling loop and exits when ctx is cancelled.
func (a *DBAggregator) Run(ctx context.Context) {
	if err := a.ProcessOnce(ctx); err != nil {
		log.Printf("[aggregation] process error: %v", err)
	}

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.ProcessOnce(ctx); err != nil {
				log.Printf("[aggregation] process error: %v", err)
			}
		}
	}
}

// ProcessOnce processes a single locked DB batch. Rows are marked processed
// only when all grouped ML payloads publish successfully.
func (a *DBAggregator) ProcessOnce(ctx context.Context) error {
	return db.ProcessUnaggregatedEvents(ctx, a.gdb, a.batchSize, func(events []db.Event) error {
		groups := make(map[aggregationKey][]SensorEvent)

		for _, row := range events {
			var reading SensorEvent
			if err := json.Unmarshal(row.Payload, &reading); err != nil {
				return fmt.Errorf("decode event id=%d: %w", row.ID, err)
			}

			// Treat the DB row tenant as authoritative for isolation.
			reading.TenantID = row.TenantID
			key := aggregationKey{tenantID: row.TenantID, deviceID: reading.DeviceID}
			groups[key] = append(groups[key], reading)
		}

		for key, readings := range groups {
			if len(readings) == 0 {
				continue
			}

			payload := MLRequest{
				DeviceID: key.deviceID,
				TenantID: key.tenantID,
				Features: Compute(readings),
			}
			if err := a.sink.Send(ctx, payload); err != nil {
				return fmt.Errorf("publish features tenant=%q device=%d readings=%d: %w", key.tenantID, key.deviceID, len(readings), err)
			}
			log.Printf("[aggregation] published tenant=%q device=%d readings=%d", key.tenantID, key.deviceID, len(readings))
		}

		log.Printf("[aggregation] processed rows=%d groups=%d", len(events), len(groups))
		return nil
	})
}
