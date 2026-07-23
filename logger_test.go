package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentWriteAndRotation(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "log_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "test.log")
	// 10KB limit
	maxSize := int64(10 * 1024)

	logger, err := NewRotateLogger(logFile, maxSize)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	numGoroutines := 100
	writesPerGoroutine := 1000 // 100 * 1000 = 100,000 total writes
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*writesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(gId int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				entry := fmt.Sprintf("log-entry-%d-%d\n", gId, j)
				_, err := logger.Write([]byte(entry))
				if err != nil {
					errChan <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Assert no write errors
	for err := range errChan {
		t.Errorf("write error occurred: %v", err)
	}

	// Close logger to flush/close active file
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	// Read all files in tempDir
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	foundEntries := make(map[string]bool)
	totalLines := 0

	for _, fileInfo := range files {
		filePath := filepath.Join(tempDir, fileInfo.Name())
		f, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("failed to open log file %s: %v", filePath, err)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "log-entry-") {
				t.Errorf("unexpected log line: %s", line)
				continue
			}
			if foundEntries[line] {
				t.Errorf("duplicate log entry found: %s", line)
			}
			foundEntries[line] = true
			totalLines++
		}
		f.Close()
	}

	expectedTotal := numGoroutines * writesPerGoroutine
	if totalLines != expectedTotal {
		t.Errorf("expected %d total lines, got %d", expectedTotal, totalLines)
	}

	// Verify all expected entries are present
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < writesPerGoroutine; j++ {
			entry := fmt.Sprintf("log-entry-%d-%d", i, j)
			if !foundEntries[entry] {
				t.Errorf("missing entry: %s", entry)
			}
		}
	}
}
