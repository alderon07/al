package main

import (
	"context"
	"strings"
	"time"
)

func managedGitContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Minute)
}
func managedGitText(b []byte) string { return strings.TrimSpace(string(b)) }
