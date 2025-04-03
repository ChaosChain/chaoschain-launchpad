package core

import (
	"bytes"
	"encoding/json"
	"sync"
)

// Database represents a simple in-memory database using string buffer
type Database struct {
	buffer map[string]*bytes.Buffer
	mu     sync.RWMutex
}

// NewDatabase creates a new in-memory database
func NewDatabase() *Database {
	return &Database{
		buffer: make(map[string]*bytes.Buffer),
	}
}

// Set stores a value in the database
func (db *Database) Set(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.buffer[key] == nil {
		db.buffer[key] = &bytes.Buffer{}
	}
	db.buffer[key].Reset()
	db.buffer[key].Write(data)
	return nil
}

// Get retrieves a value from the database
func (db *Database) Get(key string, value interface{}) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if buf := db.buffer[key]; buf != nil {
		return json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(value)
	}
	return nil
}

// Delete removes a key from the database
func (db *Database) Delete(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.buffer, key)
}
