// Package id generates the identifiers used for tasks, goals, and comments.
//
// Identifiers are a short prefix followed by a UUIDv8 (RFC 9562). The
// UUIDv8 layout mirrors UUIDv7 (time-ordered, so string comparison sorts by
// creation time) but uses the vendor-specific version nibble 8 instead of 7:
//
//	bits 0-47   (48 bits) unix time in milliseconds, big-endian
//	bits 48-51  (4 bits)  version, fixed to 0b1000 (8)
//	bits 52-63  (12 bits) random, avoids collisions within the same millisecond
//	bits 64-65  (2 bits)  variant, fixed to 0b10 (RFC 9562)
//	bits 66-127 (62 bits) cryptographically random
package id

import (
	"crypto/rand"
	"fmt"
)

const (
	// TaskPrefix identifies task IDs.
	TaskPrefix = "atm-"
	// GoalPrefix identifies goal IDs.
	GoalPrefix = "atm-g-"
	// CommentPrefix identifies comment IDs.
	CommentPrefix = "atm-c-"
)

// nowMilli returns the current Unix time in milliseconds. It is a variable so
// tests can override it deterministically.
var nowMilli = defaultNowMilli

// NewTask returns a new task ID.
func NewTask() string {
	return TaskPrefix + newUUIDv8()
}

// NewGoal returns a new goal ID.
func NewGoal() string {
	return GoalPrefix + newUUIDv8()
}

// NewComment returns a new comment ID.
func NewComment() string {
	return CommentPrefix + newUUIDv8()
}

// newUUIDv8 generates a UUIDv8 string per the layout documented above.
func newUUIDv8() string {
	var b [16]byte

	ms := uint64(nowMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	var rnd [10]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		// crypto/rand.Read on darwin/linux only fails if the OS RNG is
		// unavailable, which is unrecoverable for a program relying on it.
		panic(fmt.Sprintf("id: reading random bytes: %v", err))
	}

	b[6] = 0x80 | (rnd[0] & 0x0F) // version 1000 + top 4 bits of rand_a
	b[7] = rnd[1]                 // low 8 bits of rand_a
	b[8] = 0x80 | (rnd[2] & 0x3F) // variant 10 + top 6 bits of rand_b
	copy(b[9:16], rnd[3:10])      // remaining 56 bits of rand_b

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
