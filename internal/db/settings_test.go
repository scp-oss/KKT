package db

import "testing"

func TestVisibleColumnsDefaultToShown(t *testing.T) {
	store := newTestDB(t)

	visible, err := store.GetVisibleColumns()
	if err != nil {
		t.Fatalf("GetVisibleColumns: %v", err)
	}
	for _, c := range DashboardColumns {
		if !visible[c.Key] {
			t.Errorf("column %q should default to visible before any setting is saved", c.Key)
		}
	}
}

func TestSetVisibleColumnsHidesUnlisted(t *testing.T) {
	store := newTestDB(t)

	// Simulate a form submit where only "organization" and "ofd" were
	// checked - everything else must end up hidden, not left as-is.
	if err := store.SetVisibleColumns(map[string]bool{"organization": true, "ofd": true}); err != nil {
		t.Fatalf("SetVisibleColumns: %v", err)
	}

	visible, err := store.GetVisibleColumns()
	if err != nil {
		t.Fatalf("GetVisibleColumns: %v", err)
	}
	for _, c := range DashboardColumns {
		want := c.Key == "organization" || c.Key == "ofd"
		if visible[c.Key] != want {
			t.Errorf("column %q visible = %v, want %v", c.Key, visible[c.Key], want)
		}
	}
}
