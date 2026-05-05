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

type DBWorkerConfig struct {
	Interval  time.Duration
	BatchSize int
}

type DBWorker struct {
	gdb    *gorm.DB
	sink   FeatureSink
	config DBWorkerConfig
}

func NewDBWorker(gdb *gorm.DB, sink FeatureSink, config DBWorkerConfig) *DBWorker {
	if config.Interval <= 0 {
		config.Interval = 5 * time.Second
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 5000
	}

	return &DBWorker{
		gdb:    gdb,
		sink:   sink,
		config: config,
	}
}

func (w *DBWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	log.Printf("[db-worker] polling unprocessed events every %s (batch=%d)", w.config.Interval, w.config.BatchSize)

	for {
		select {
		case <-ctx.Done():
			log.Println("[db-worker] stopping")
			return
		case <-ticker.C:
			if err := w.ProcessBatch(ctx); err != nil {
				log.Printf("[db-worker] process batch error: %v", err)
			}
		}
	}
}

func (w *DBWorker) ProcessBatch(ctx context.Context) error {
	events, err := db.GetUnprocessedEvents(ctx, w.gdb, w.config.BatchSize)
	if err != nil {
		return fmt.Errorf("load unprocessed events: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	groups := groupEvents(events)
	log.Printf("[db-worker] loaded %d events across %d groups", len(events), len(groups))

	for _, group := range groups {
		features := Compute(group.readings)
		payload := MLRequest{
			DeviceID: group.deviceID,
			TenantID: group.tenantID,
			Features: features,
		}

		if err := w.sink.Send(ctx, payload); err != nil {
			log.Printf("[db-worker] send failed tenant=%q device=%q events=%d: %v", group.tenantID, group.deviceID, len(group.ids), err)
			continue
		}

		if err := db.MarkEventsProcessed(ctx, w.gdb, group.ids, time.Now().UTC()); err != nil {
			log.Printf("[db-worker] mark processed failed tenant=%q device=%q events=%d: %v", group.tenantID, group.deviceID, len(group.ids), err)
			continue
		}

		log.Printf("[db-worker] sent features tenant=%q device=%q events=%d", group.tenantID, group.deviceID, len(group.ids))
	}

	return nil
}

type eventGroup struct {
	tenantID string
	deviceID string
	ids      []int64
	readings []SensorEvent
}

func groupEvents(events []db.Event) []eventGroup {
	groupsByKey := make(map[string]*eventGroup)
	orderedKeys := make([]string, 0)

	for _, stored := range events {
		var reading SensorEvent
		if err := json.Unmarshal(stored.Payload, &reading); err != nil {
			log.Printf("[db-worker] skipping malformed stored event id=%d: %v", stored.ID, err)
			continue
		}

		if reading.TenantID == "" || reading.DeviceID == "" {
			log.Printf("[db-worker] skipping event id=%d with missing tenant_id or device_id", stored.ID)
			continue
		}

		key := reading.TenantID + "\x00" + reading.DeviceID
		group, ok := groupsByKey[key]
		if !ok {
			group = &eventGroup{
				tenantID: reading.TenantID,
				deviceID: reading.DeviceID,
			}
			groupsByKey[key] = group
			orderedKeys = append(orderedKeys, key)
		}

		group.ids = append(group.ids, stored.ID)
		group.readings = append(group.readings, reading)
	}

	groups := make([]eventGroup, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		groups = append(groups, *groupsByKey[key])
	}
	return groups
}
