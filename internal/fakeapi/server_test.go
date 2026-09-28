package fakeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func get(t *testing.T, s *Server, path string, knobs string) (*httptest.ResponseRecorder, Page) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if knobs != "" {
		req.Header.Set("X-Fakeapi-Knobs", knobs)
	}
	rr := httptest.NewRecorder()
	s.Records(rr, req)
	if rr.Code != http.StatusOK {
		return rr, Page{}
	}
	var p Page
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode page: %v (body %q)", err, rr.Body.String())
	}
	return rr, p
}

func TestPagination(t *testing.T) {
	s := NewServer(1)
	_, p1 := get(t, s, "/records?limit=5", `{"max_records":20}`)
	if len(p1.Records) != 5 {
		t.Fatalf("page1: got %d records, want 5", len(p1.Records))
	}
	if p1.Records[0].ID != 1 || p1.Records[4].ID != 5 {
		t.Fatalf("page1 ids = %d..%d, want 1..5", p1.Records[0].ID, p1.Records[4].ID)
	}
	if p1.NextCursor != 5 || !p1.HasMore {
		t.Fatalf("page1 next_cursor=%d has_more=%v, want 5/true", p1.NextCursor, p1.HasMore)
	}

	_, p2 := get(t, s, "/records?limit=5&cursor=5", `{"max_records":20}`)
	if p2.Records[0].ID != 6 || p2.Records[4].ID != 10 {
		t.Fatalf("page2 ids = %d..%d, want 6..10", p2.Records[0].ID, p2.Records[4].ID)
	}
}

func TestMaxRecordsTerminates(t *testing.T) {
	s := NewServer(2)
	_, p := get(t, s, "/records?limit=8", `{"max_records":10}`)
	if len(p.Records) != 8 || !p.HasMore {
		t.Fatalf("want 8 records has_more=true, got %d/%v", len(p.Records), p.HasMore)
	}
	_, p2 := get(t, s, "/records?limit=8&cursor=8", `{"max_records":10}`)
	if len(p2.Records) != 2 || p2.HasMore {
		t.Fatalf("want 2 records has_more=false, got %d/%v", len(p2.Records), p2.HasMore)
	}
}

func TestRetryAfter(t *testing.T) {
	s := NewServer(3)
	rr, _ := get(t, s, "/records?limit=5", `{"error_rate":1,"error_class":"429","retry_after_seconds":7}`)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "7" {
		t.Fatalf("Retry-After = %q, want 7", got)
	}
}

func TestCursorExpiry(t *testing.T) {
	s := NewServer(4)
	_, p1 := get(t, s, "/records?limit=1", `{"cursor_expiry_after_requests":1}`)
	_, p2 := get(t, s, "/records?limit=1&cursor="+strconv.FormatInt(p1.NextCursor, 10), `{"cursor_expiry_after_requests":1}`)
	if p2.NextCursor == 0 {
		t.Fatal("second request should succeed and advance the cursor")
	}
	rr, _ := get(t, s, "/records?limit=1&cursor="+strconv.FormatInt(p1.NextCursor, 10), `{"cursor_expiry_after_requests":1}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 cursor expired on stale cursor reuse", rr.Code)
	}
}

func TestDuplicate(t *testing.T) {
	s := NewServer(5)
	_, p := get(t, s, "/records?limit=50", `{"duplicate_probability":1}`)
	seen := map[int64]bool{}
	for _, r := range p.Records {
		if seen[r.ID] {
			return
		}
		seen[r.ID] = true
	}
	t.Fatal("expected a duplicate id with duplicate_probability=1")
}

func TestOutOfOrder(t *testing.T) {
	s := NewServer(6)
	_, p := get(t, s, "/records?limit=50", `{"out_of_order_probability":1}`)
	for i := 1; i < len(p.Records); i++ {
		if p.Records[i].ID < p.Records[i-1].ID {
			return
		}
	}
	t.Fatal("expected an out-of-order id with out_of_order_probability=1")
}

func TestPoisonRecord(t *testing.T) {
	s := NewServer(7)
	_, p := get(t, s, "/records?limit=5", `{"poison_id":3}`)
	for _, r := range p.Records {
		if r.ID == 3 && r.Data != "poison" {
			t.Fatalf("poison record data = %q, want \"poison\"", r.Data)
		}
	}
}
