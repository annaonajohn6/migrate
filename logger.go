package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// RotateLogger is a thread-safe writer that wraps a file and rotates it when it exceeds maxSize.
type RotateLogger struct {
	mu       sync.Mutex
	filename string
	maxSize  int64
	file     *os.File
	size     int64
}

// NewRotateLogger creates a new RotateLogger.
func NewRotateLogger(filename string, maxSize int64) (*RotateLogger, error) {
	l := &RotateLogger{
		filename: filename,
		maxSize:  maxSize,
	}
	if err := l.openNew(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *RotateLogger) openNew() error {
	file, err := os.OpenFile(l.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	l.file = file
	l.size = info.Size()
	return nil
}

// Write writes data to the log file, rotating it first if the write would exceed maxSize.
func (l *RotateLogger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	writeLen := int64(len(p))
	if l.size+writeLen > l.maxSize {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.size += int64(n)
	return n, err
}

func (l *RotateLogger) rotate() error {
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return err
		}
		l.file = nil
	}

	// Rename current file to a unique backup name
	backupName := fmt.Sprintf("%s.%d", l.filename, time.Now().UnixNano())
	for i := 0; ; i++ {
		if _, err := os.Stat(backupName); os.IsNotExist(err) {
			break
		}
		backupName = fmt.Sprintf("%s.%d-%d", l.filename, time.Now().UnixNano(), i)
	}

	if err := os.Rename(l.filename, backupName); err != nil {
		return err
	}

	return l.openNew()
}

// Close closes the active log file.
func (l *RotateLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}
