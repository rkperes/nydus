package fakeapi

type Knobs struct {
	MaxRecords        int64   `json:"max_records"`
	LatencyMS         int     `json:"latency_ms"`
	LatencyTailMS     int     `json:"latency_tail_ms"`
	ErrorRate         float64 `json:"error_rate"`
	ErrorClass        string  `json:"error_class"`
	RetryAfterSeconds int     `json:"retry_after_seconds"`
	CursorExpiryAfter int     `json:"cursor_expiry_after_requests"`
	ClockSkewSeconds  int     `json:"clock_skew_seconds"`
	DuplicateProb     float64 `json:"duplicate_probability"`
	OutOfOrderProb    float64 `json:"out_of_order_probability"`
	PoisonID          int64   `json:"poison_id"`
}
