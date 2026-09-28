package fakeapi

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	mu       sync.Mutex
	rngMu    sync.Mutex
	rng      *rand.Rand
	knobs    Knobs
	reqCount int64
	issuedAt map[int64]int64
}

func NewServer(seed int64) *Server {
	return &Server{
		rng:      rand.New(rand.NewSource(seed)),
		issuedAt: make(map[int64]int64),
	}
}

func (s *Server) float() float64 {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Float64()
}

func (s *Server) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) Control(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var k Knobs
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		http.Error(w, "invalid knobs: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.knobs = k
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(k)
}

func (s *Server) Records(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.reqCount++
	requestNo := s.reqCount
	knobs := s.knobs
	s.mu.Unlock()

	if h := r.Header.Get("X-Fakeapi-Knobs"); h != "" {
		if err := json.Unmarshal([]byte(h), &knobs); err != nil {
			http.Error(w, "invalid X-Fakeapi-Knobs: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if knobs.ErrorRate > 0 && s.float() < knobs.ErrorRate {
		s.injectError(w, knobs)
		return
	}

	if knobs.LatencyMS > 0 || knobs.LatencyTailMS > 0 {
		s.applyLatency(knobs)
	}

	cursor := int64(0)
	if v := r.URL.Query().Get("cursor"); v != "" {
		var err error
		cursor, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if n < 1 {
			n = 1
		}
		if n > 1000 {
			n = 1000
		}
		limit = n
	}

	if knobs.CursorExpiryAfter > 0 {
		s.mu.Lock()
		issued, ok := s.issuedAt[cursor]
		s.mu.Unlock()
		if ok && requestNo-issued > int64(knobs.CursorExpiryAfter) {
			http.Error(w, `{"error":"cursor expired"}`, http.StatusBadRequest)
			return
		}
	}

	var modifiedSince time.Time
	var hasModifiedSince bool
	if v := r.URL.Query().Get("modified_since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			http.Error(w, "invalid modified_since", http.StatusBadRequest)
			return
		}
		modifiedSince = t
		hasModifiedSince = true
	}

	page := s.generatePage(cursor, limit, knobs, modifiedSince, hasModifiedSince)

	if knobs.CursorExpiryAfter > 0 {
		s.mu.Lock()
		s.issuedAt[page.NextCursor] = requestNo
		s.mu.Unlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(page)
}

func (s *Server) injectError(w http.ResponseWriter, knobs Knobs) {
	switch knobs.ErrorClass {
	case "429":
		if knobs.RetryAfterSeconds > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(knobs.RetryAfterSeconds))
		}
		http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
	case "timeout":
		time.Sleep(120 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	default:
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}
}

func (s *Server) applyLatency(knobs Knobs) {
	delay := time.Duration(knobs.LatencyMS) * time.Millisecond
	if knobs.LatencyTailMS > 0 {
		delay += time.Duration(s.float()*float64(knobs.LatencyTailMS)) * time.Millisecond
	}
	time.Sleep(delay)
}

func (s *Server) generatePage(cursor int64, limit int, knobs Knobs, since time.Time, hasSince bool) Page {
	skew := time.Duration(knobs.ClockSkewSeconds) * time.Second

	start := cursor + 1
	end := start + int64(limit) - 1
	if knobs.MaxRecords > 0 {
		if start > knobs.MaxRecords {
			return Page{Records: []Record{}, NextCursor: cursor, HasMore: false}
		}
		if end > knobs.MaxRecords {
			end = knobs.MaxRecords
		}
	}

	records := make([]Record, 0, limit)
	count := end - start + 1
	for i := int64(0); i < count; i++ {
		id := start + i
		if knobs.DuplicateProb > 0 && len(records) > 0 && s.float() < knobs.DuplicateProb {
			records = append(records, records[len(records)-1])
			continue
		}
		if knobs.OutOfOrderProb > 0 && len(records) > 0 && s.float() < knobs.OutOfOrderProb {
			id = records[len(records)-1].ID - 1
		}
		if hasSince && modifiedAt(id, skew).Before(since) {
			continue
		}
		r := Record{
			ID:         id,
			ModifiedAt: modifiedAt(id, skew).Format(time.RFC3339),
			Data:       "record-" + strconv.FormatInt(id, 10),
		}
		if knobs.PoisonID != 0 && id == knobs.PoisonID {
			r.Data = "poison"
		}
		records = append(records, r)
	}

	hasMore := true
	if knobs.MaxRecords > 0 {
		hasMore = end < knobs.MaxRecords
	}
	return Page{Records: records, NextCursor: end, HasMore: hasMore}
}
