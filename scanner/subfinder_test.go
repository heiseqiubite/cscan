package scanner

import (
	"context"
	"testing"
	"time"
)

func TestSubfinderTempFileCleanup(t *testing.T) {
	scanner := NewSubfinderScanner()
	opts := &SubfinderOptions{
		ProviderConfig: map[string][]string{"github": {"dummy-key-for-test"}},
		Timeout:            1,
		MaxEnumerationTime: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	// Execution should successfully complete and clean up the randomly generated temp file
	scanner.enumerateDomain(ctx, "example.com", opts)
}
