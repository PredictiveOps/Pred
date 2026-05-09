package db_test

import (
	"context"
	"testing"

	"notifications-service/db"

	"testutil"
)

func TestInsertNotification(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)
	ctx := context.Background()

	payload := []byte(`{"title":"hello","body":"world"}`)
	id, err := db.InsertNotification(ctx, gdb, "tenant-1", "email", payload)
	if err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero notification ID")
	}

	var n db.Notification
	if err := gdb.First(&n, id).Error; err != nil {
		t.Fatalf("fetch notification: %v", err)
	}
	if n.TenantID != "tenant-1" {
		t.Errorf("tenant_id: got %q, want %q", n.TenantID, "tenant-1")
	}
	if n.Type != "email" {
		t.Errorf("type: got %q, want %q", n.Type, "email")
	}
}

func TestInsertDelivery(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)
	ctx := context.Background()

	notifID, err := db.InsertNotification(ctx, gdb, "tenant-2", "email", []byte(`{}`))
	if err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}

	deliveryID, err := db.InsertDelivery(ctx, gdb, notifID, "tenant-2", "user-42", "user@example.com", nil)
	if err != nil {
		t.Fatalf("InsertDelivery: %v", err)
	}
	if deliveryID == 0 {
		t.Fatal("expected non-zero delivery ID")
	}

	var d db.NotificationDelivery
	if err := gdb.First(&d, deliveryID).Error; err != nil {
		t.Fatalf("fetch delivery: %v", err)
	}
	if d.UserID != "user-42" {
		t.Errorf("user_id: got %q, want %q", d.UserID, "user-42")
	}
	if d.Recipient != "user@example.com" {
		t.Errorf("recipient: got %q, want %q", d.Recipient, "user@example.com")
	}
	if d.Status != "pending" {
		t.Errorf("initial status: got %q, want %q", d.Status, "pending")
	}
}

func TestUpdateDeliveryStatus(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)
	ctx := context.Background()

	notifID, err := db.InsertNotification(ctx, gdb, "tenant-3", "push", []byte(`{}`))
	if err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}
	deliveryID, err := db.InsertDelivery(ctx, gdb, notifID, "tenant-3", "user-7", "device-token-abc", nil)
	if err != nil {
		t.Fatalf("InsertDelivery: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		if err := db.UpdateDeliveryStatus(ctx, gdb, deliveryID, "delivered", ""); err != nil {
			t.Fatalf("UpdateDeliveryStatus: %v", err)
		}
		var d db.NotificationDelivery
		gdb.First(&d, deliveryID)
		if d.Status != "delivered" {
			t.Errorf("status: got %q, want %q", d.Status, "delivered")
		}
		if d.Error != nil {
			t.Errorf("error field should be nil on success, got %q", *d.Error)
		}
	})

	t.Run("failure", func(t *testing.T) {
		if err := db.UpdateDeliveryStatus(ctx, gdb, deliveryID, "failed", "connection refused"); err != nil {
			t.Fatalf("UpdateDeliveryStatus: %v", err)
		}
		var d db.NotificationDelivery
		gdb.First(&d, deliveryID)
		if d.Status != "failed" {
			t.Errorf("status: got %q, want %q", d.Status, "failed")
		}
		if d.Error == nil || *d.Error != "connection refused" {
			t.Errorf("error field: got %v, want %q", d.Error, "connection refused")
		}
	})
}

func TestGetNotifications(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)
	ctx := context.Background()

	// Insert 3 notifications for tenant A (in order so created_at differs).
	var insertedIDs []int64
	for i, typ := range []string{"email", "push", "email"} {
		id, err := db.InsertNotification(ctx, gdb, "tenant-gn-a", typ, []byte(`{"i":`+string(rune('0'+i))+`}`))
		if err != nil {
			t.Fatalf("InsertNotification %d: %v", i, err)
		}
		insertedIDs = append(insertedIDs, id)
	}
	// Insert 1 notification for tenant B.
	if _, err := db.InsertNotification(ctx, gdb, "tenant-gn-b", "push", []byte(`{}`)); err != nil {
		t.Fatalf("InsertNotification tenant-b: %v", err)
	}

	t.Run("tenant isolation", func(t *testing.T) {
		got, err := db.GetNotifications(ctx, gdb, "tenant-gn-a", 10)
		if err != nil {
			t.Fatalf("GetNotifications: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("count: got %d, want 3", len(got))
		}
		for _, n := range got {
			if n.TenantID != "tenant-gn-a" {
				t.Errorf("cross-tenant leak: got tenant_id %q", n.TenantID)
			}
		}
	})

	t.Run("descending order", func(t *testing.T) {
		got, err := db.GetNotifications(ctx, gdb, "tenant-gn-a", 10)
		if err != nil {
			t.Fatalf("GetNotifications: %v", err)
		}
		for i := 1; i < len(got); i++ {
			if got[i].CreatedAt.After(got[i-1].CreatedAt) {
				t.Errorf("not DESC at index %d: %v > %v", i, got[i].CreatedAt, got[i-1].CreatedAt)
			}
		}
		// Most-recently inserted row should come first.
		if got[0].ID != insertedIDs[len(insertedIDs)-1] {
			t.Errorf("first row ID: got %d, want %d", got[0].ID, insertedIDs[len(insertedIDs)-1])
		}
	})

	t.Run("limit", func(t *testing.T) {
		got, err := db.GetNotifications(ctx, gdb, "tenant-gn-a", 2)
		if err != nil {
			t.Fatalf("GetNotifications: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("count with limit=2: got %d, want 2", len(got))
		}
	})
}

func TestDeviceTokensForUsers(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)
	ctx := context.Background()

	tokens := []db.DeviceToken{
		{TenantID: "tenant-tok", UserID: "u1", Token: "tok-u1-ios", Platform: "ios"},
		{TenantID: "tenant-tok", UserID: "u2", Token: "tok-u2-android", Platform: "android"},
		{TenantID: "other-tenant", UserID: "u1", Token: "tok-other", Platform: "ios"},
	}
	for i := range tokens {
		if err := gdb.Create(&tokens[i]).Error; err != nil {
			t.Fatalf("seed device token: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, tok := range tokens {
			gdb.Delete(&db.DeviceToken{}, tok.ID)
		}
	})

	got, err := db.DeviceTokensForUsers(ctx, gdb, "tenant-tok", []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("DeviceTokensForUsers: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("count: got %d, want 2", len(got))
	}
	for _, tok := range got {
		if tok.TenantID != "tenant-tok" {
			t.Errorf("wrong tenant returned: %q", tok.TenantID)
		}
	}

	// Verify tokens from another tenant are not returned.
	got, err = db.DeviceTokensForUsers(ctx, gdb, "tenant-tok", []string{"u1"})
	if err != nil {
		t.Fatalf("DeviceTokensForUsers: %v", err)
	}
	if len(got) != 1 || got[0].Token != "tok-u1-ios" {
		t.Errorf("expected only u1's token for tenant-tok, got %+v", got)
	}
}
