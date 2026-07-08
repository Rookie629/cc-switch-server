package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	ErrProviderNotFound      = errors.New("provider not found")
	ErrProviderActive        = errors.New("cannot delete active provider")
	ErrProviderAlreadyExists = errors.New("provider name already exists")
)

// Store handles JSON file persistence with file locking.
type Store struct {
	mu   sync.Mutex
	dir  string
	file string
}

// New creates a new Store, ensuring the data directory exists.
func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "backups"), 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &Store{
		dir:  dataDir,
		file: filepath.Join(dataDir, "providers.json"),
	}, nil
}

// DefaultDataDir returns ~/.cc-switch-server.
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", "cc-switch-server")
	}
	return filepath.Join(home, ".cc-switch-server")
}

// lockFile acquires an exclusive file lock (flock).
func (s *Store) lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlockFile releases the lock.
func (s *Store) unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// Read loads providers from the JSON file. Returns an empty state if the file
// doesn't exist.
func (s *Store) Read() (*ProvidersFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.file, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("open providers file: %w", err)
	}
	defer f.Close()

	if err := s.lockFile(f); err != nil {
		return nil, fmt.Errorf("lock file: %w", err)
	}
	defer s.unlockFile(f)

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	if stat.Size() == 0 {
		return &ProvidersFile{
			Version:   1,
			Providers: []Provider{},
		}, nil
	}

	var pf ProvidersFile
	if err := json.NewDecoder(f).Decode(&pf); err != nil {
		// File is corrupted — try backup recovery
		backup, backupErr := s.loadLatestBackup()
		if backupErr != nil {
			return &ProvidersFile{Version: 1, Providers: []Provider{}}, nil
		}
		// Restore from backup
		s.writeUnsafe(backup)
		return backup, nil
	}

	if pf.Providers == nil {
		pf.Providers = []Provider{}
	}
	return &pf, nil
}

// Write persists providers to the JSON file with atomic write (temp + rename).
func (s *Store) Write(pf *ProvidersFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Backup before write
	s.backupUnsafe()

	return s.writeUnsafe(pf)
}

// writeUnsafe writes without acquiring the mutex or creating a backup.
func (s *Store) writeUnsafe(pf *ProvidersFile) error {
	tmpFile := s.file + ".tmp"

	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(pf); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("encode providers: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpFile, s.file); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// backupUnsafe creates a timestamped backup (called under lock).
func (s *Store) backupUnsafe() {
	backupDir := filepath.Join(s.dir, "backups")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("providers_%s.json", time.Now().Format("20060102_150405")))

	data, err := os.ReadFile(s.file)
	if err != nil {
		return
	}
	os.WriteFile(backupFile, data, 0600)

	// Rotate: keep only 10 most recent backups
	entries, _ := os.ReadDir(backupDir)
	if len(entries) > 10 {
		// Simple rotation — remove oldest
		for i := 0; i < len(entries)-10; i++ {
			os.Remove(filepath.Join(backupDir, entries[i].Name()))
		}
	}
}

// loadLatestBackup tries to load the newest backup file.
func (s *Store) loadLatestBackup() (*ProvidersFile, error) {
	backupDir := filepath.Join(s.dir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		return nil, errors.New("no backups available")
	}

	// Iterate in reverse (newest first)
	for i := len(entries) - 1; i >= 0; i-- {
		data, err := os.ReadFile(filepath.Join(backupDir, entries[i].Name()))
		if err != nil {
			continue
		}
		var pf ProvidersFile
		if json.Unmarshal(data, &pf) == nil {
			return &pf, nil
		}
	}
	return nil, errors.New("all backups corrupted")
}

// ----- convenience helpers for service layer -----

// FindByName returns a provider by name (case-sensitive).
func FindByName(pf *ProvidersFile, name string) (*Provider, int) {
	for i, p := range pf.Providers {
		if p.Name == name {
			return &pf.Providers[i], i
		}
	}
	return nil, -1
}

// FindByID returns a provider by ID.
func FindByID(pf *ProvidersFile, id string) (*Provider, int) {
	for i, p := range pf.Providers {
		if p.ID == id {
			return &pf.Providers[i], i
		}
	}
	return nil, -1
}

// ActiveProvider returns the currently active provider, or nil.
func ActiveProvider(pf *ProvidersFile) *Provider {
	if pf.ActiveProviderID == "" {
		return nil
	}
	p, _ := FindByID(pf, pf.ActiveProviderID)
	return p
}
