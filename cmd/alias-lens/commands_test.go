package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCommandOutputStopsAtDeadline(t *testing.T) {
	started := time.Now()
	_, err := commandOutput(context.Background(), 50*time.Millisecond, "sleep", "5")
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("deadline error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("deadline took %s", elapsed)
	}
}
