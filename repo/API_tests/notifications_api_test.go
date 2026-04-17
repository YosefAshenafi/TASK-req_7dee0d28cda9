package api_tests

import (
	"context"
	"net/http"
	"testing"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// seedNotificationForAdmin inserts a notification owned by the admin user and
// returns its ID string. We obtain the admin's UUID from GET /api/v1/auth/me.
func seedNotificationForAdmin(t *testing.T) string {
	t.Helper()

	// Get admin's UUID so the notification is scoped to the correct user.
	rr := admin(http.MethodGet, "/api/v1/auth/me", nil)
	mustStatus(t, rr, http.StatusOK)
	me := decodeJSON(t, rr)
	adminIDStr, ok := me["id"].(string)
	if !ok || adminIDStr == "" {
		t.Fatal("could not retrieve admin user ID from /api/v1/auth/me")
	}

	ctx := context.Background()
	notifRepo := repository.NewNotificationRepository(testPool)
	title := "test-notif"
	created, err := notifRepo.Create(ctx, &domain.Notification{
		UserID:  parseUUID(t, adminIDStr),
		Title:   title,
		Context: []byte("{}"),
	})
	if err != nil {
		t.Fatalf("seed notification: %v", err)
	}
	return created.ID.String()
}

// ── GET /api/v1/notifications ────────────────────────────────────────────────

func TestNotificationsList(t *testing.T) {
	notifID := seedNotificationForAdmin(t)

	rr := admin(http.MethodGet, "/api/v1/notifications", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)

	if body["page"] == nil {
		t.Error("response missing 'page' field")
	}
	if body["total"] == nil {
		t.Error("response missing 'total' field")
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("response missing 'items' array")
	}

	found := false
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["id"] == notifID {
			found = true
			if row["title"] == nil {
				t.Error("notification 'title' missing from list item")
			}
			if row["is_read"] == nil {
				t.Error("notification 'is_read' missing from list item")
			}
		}
	}
	if !found {
		t.Errorf("seeded notification %s not found in list (total=%v)", notifID, body["total"])
	}
}

func TestNotificationsList_UnreadFilter(t *testing.T) {
	seedNotificationForAdmin(t)

	rr := admin(http.MethodGet, "/api/v1/notifications?is_read=false", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	for _, raw := range body["items"].([]any) {
		row, _ := raw.(map[string]any)
		if isRead, _ := row["is_read"].(bool); isRead {
			t.Error("is_read=false filter returned a read notification")
		}
	}
}

// ── PUT /api/v1/notifications/:id/read ───────────────────────────────────────

func TestNotificationsMarkRead(t *testing.T) {
	notifID := seedNotificationForAdmin(t)

	rr := admin(http.MethodPut, "/api/v1/notifications/"+notifID+"/read", nil)
	mustStatus(t, rr, http.StatusNoContent)

	// Confirm it no longer appears in the unread list.
	rr = admin(http.MethodGet, "/api/v1/notifications?is_read=false", nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	for _, raw := range body["items"].([]any) {
		row, _ := raw.(map[string]any)
		if row["id"] == notifID {
			t.Errorf("notification %s still appears in unread list after MarkRead", notifID)
		}
	}
}

func TestNotificationsMarkRead_InvalidID(t *testing.T) {
	rr := admin(http.MethodPut, "/api/v1/notifications/not-a-uuid/read", nil)
	mustStatus(t, rr, http.StatusBadRequest)
}
