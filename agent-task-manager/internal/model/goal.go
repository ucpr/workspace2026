package model

import "time"

// Goal bundles one or more tasks under a shared objective. Tasks may belong
// to multiple goals (requirements §6, §10).
type Goal struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	TaskIDs     []string  `yaml:"task_ids,omitempty"`
	CreatedAt   time.Time `yaml:"created_at"`
	UpdatedAt   time.Time `yaml:"updated_at"`
}
