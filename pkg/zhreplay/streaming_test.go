package zhreplay

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
)

func TestStreamReplay(t *testing.T) {
	// Initialize stores for testing
	objDataPath := "./inizh/Data/INI"
	objectStore, err := iniparse.NewObjectStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load object store: %v", err)
	}

	powerStore, err := iniparse.NewPowerStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load power store: %v", err)
	}

	upgradeStore, err := iniparse.NewUpgradeStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load upgrade store: %v", err)
	}

	// Test with a real replay file
	replayFile := "./example/simple-generals-replay.rep"
	if _, err := os.Stat(replayFile); os.IsNotExist(err) {
		t.Skipf("Skipping test: replay file not found: %s", replayFile)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test streaming with default options
	bodyChan, streamingReplay, err := StreamReplay(ctx, replayFile, objectStore, powerStore, upgradeStore, nil)
	if err != nil {
		t.Fatalf("StreamReplay failed: %v", err)
	}

	// Verify we got a streaming replay with header
	if streamingReplay == nil {
		t.Fatal("Expected streaming replay, got nil")
	}
	if streamingReplay.Header == nil {
		t.Fatal("Expected header, got nil")
	}

	// Collect some body chunks
	var chunks []*body.BodyChunk
	timeout := time.After(5 * time.Second)

	for {
		select {
		case chunk, ok := <-bodyChan:
			if !ok {
				// Channel closed, we're done
				goto done
			}
			chunks = append(chunks, chunk)
		case <-timeout:
			// Timeout - this is expected for a complete file
			goto done
		}
	}

done:
	// Verify we got some chunks
	if len(chunks) == 0 {
		t.Fatal("Expected at least one body chunk, got none")
	}

	t.Logf("Received %d body chunks", len(chunks))

	// Verify first chunk has expected structure
	firstChunk := chunks[0]
	if firstChunk.TimeCode == 0 && firstChunk.OrderCode == 0 && firstChunk.PlayerID == 0 {
		t.Error("First chunk appears to be end marker")
	}
}

func TestStreamReplayWithOptions(t *testing.T) {
	// Initialize stores for testing
	objDataPath := "./inizh/Data/INI"
	objectStore, err := iniparse.NewObjectStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load object store: %v", err)
	}

	powerStore, err := iniparse.NewPowerStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load power store: %v", err)
	}

	upgradeStore, err := iniparse.NewUpgradeStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load upgrade store: %v", err)
	}

	// Test with a real replay file
	replayFile := "./example/simple-generals-replay.rep"
	if _, err := os.Stat(replayFile); os.IsNotExist(err) {
		t.Skipf("Skipping test: replay file not found: %s", replayFile)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test streaming with custom options
	options := &StreamReplayOptions{
		PollInterval: 50 * time.Millisecond,
		MaxWaitTime:  5 * time.Second,
		BufferSize:   50,
	}

	bodyChan, streamingReplay, err := StreamReplay(ctx, replayFile, objectStore, powerStore, upgradeStore, options)
	if err != nil {
		t.Fatalf("StreamReplay failed: %v", err)
	}

	// Verify we got a streaming replay
	if streamingReplay == nil {
		t.Fatal("Expected streaming replay, got nil")
	}

	// Collect some body chunks
	var chunks []*body.BodyChunk
	timeout := time.After(3 * time.Second)

	for {
		select {
		case chunk, ok := <-bodyChan:
			if !ok {
				// Channel closed, we're done
				goto done
			}
			chunks = append(chunks, chunk)
		case <-timeout:
			// Timeout - this is expected for a complete file
			goto done
		}
	}

done:
	// Verify we got some chunks
	if len(chunks) == 0 {
		t.Fatal("Expected at least one body chunk, got none")
	}

	t.Logf("Received %d body chunks with custom options", len(chunks))
}

func TestStreamReplayContextCancellation(t *testing.T) {
	// Initialize stores for testing
	objDataPath := "./inizh/Data/INI"
	objectStore, err := iniparse.NewObjectStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load object store: %v", err)
	}

	powerStore, err := iniparse.NewPowerStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load power store: %v", err)
	}

	upgradeStore, err := iniparse.NewUpgradeStore(objDataPath)
	if err != nil {
		t.Skipf("Skipping test: could not load upgrade store: %v", err)
	}

	// Test with a real replay file
	replayFile := "./example/simple-generals-replay.rep"
	if _, err := os.Stat(replayFile); os.IsNotExist(err) {
		t.Skipf("Skipping test: replay file not found: %s", replayFile)
	}

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	bodyChan, streamingReplay, err := StreamReplay(ctx, replayFile, objectStore, powerStore, upgradeStore, nil)
	if err != nil {
		t.Fatalf("StreamReplay failed: %v", err)
	}

	// Verify we got a streaming replay
	if streamingReplay == nil {
		t.Fatal("Expected streaming replay, got nil")
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Try to read from channel - should be closed due to context cancellation
	select {
	case _, ok := <-bodyChan:
		if ok {
			t.Error("Expected channel to be closed due to context cancellation")
		}
	default:
		// Channel might still be open briefly, this is acceptable
	}
}

func TestDefaultStreamReplayOptions(t *testing.T) {
	options := DefaultStreamReplayOptions()

	if options.PollInterval != 100*time.Millisecond {
		t.Errorf("Expected PollInterval to be 100ms, got %v", options.PollInterval)
	}

	if options.MaxWaitTime != 30*time.Second {
		t.Errorf("Expected MaxWaitTime to be 30s, got %v", options.MaxWaitTime)
	}

	if options.BufferSize != 100 {
		t.Errorf("Expected BufferSize to be 100, got %d", options.BufferSize)
	}
}
