package models

import "time"

type Event struct {
	Type   string    `json:"type"`
	Serial string    `json:"serial,omitempty"`
	Time   time.Time `json:"time"`
	Data   any       `json:"data,omitempty"`
}
