package fakeapi

import "time"

type Record struct {
	ID         int64  `json:"id"`
	ModifiedAt string `json:"modified_at"`
	Data       string `json:"data"`
}

type Page struct {
	Records    []Record `json:"records"`
	NextCursor int64    `json:"next_cursor"`
	HasMore    bool     `json:"has_more"`
}

var baseTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

const idInterval = time.Minute

func modifiedAt(id int64, skew time.Duration) time.Time {
	return baseTime.Add(time.Duration(id) * idInterval).Add(skew)
}
