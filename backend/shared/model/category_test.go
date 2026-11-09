package model

import (
	"encoding/json"
	"testing"
)

// Category amounts arrive as JSON from the category-group query and leave as
// JSON to the clients. float32 cannot hold paise above ~₹1,31,072, so
// 162178.99 used to reach the UI as 162178.98 — a row that looked balanced
// while its (exactly computed) available balance read -0.01.
func TestCategoryAmountsKeepPaise(t *testing.T) {
	in := []byte(`{"budgeted":{"2023-05":162178.98},"activity":{"2023-05":-162178.99},"balance":{"2023-05":-0.01}}`)
	var c Category
	if err := json.Unmarshal(in, &c); err != nil {
		t.Fatal(err)
	}
	if c.Activity["2023-05"] != -162178.99 {
		t.Fatalf("activity = %v, want -162178.99", c.Activity["2023-05"])
	}
	if got := c.Budgeted["2023-05"] + c.Activity["2023-05"]; got > -0.0099 || got < -0.0101 {
		t.Fatalf("budgeted + activity = %v, want -0.01", got)
	}
	out, err := json.Marshal(c.Activity)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"2023-05":-162178.99}` {
		t.Fatalf("re-encoded activity = %s", out)
	}
}
