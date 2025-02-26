package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type Record struct {
	Timestamp int64        `json:"ts"`
	Data      bytes.Buffer `json:"data"`
	isNew     bool
}

type RecordHeader struct {
	hasChanged bool
	records    []Record
}

type Database struct {
	data       map[string]RecordHeader
	name       string
	ttl        int64 // hours
	stopChan   chan struct{}
	mu         sync.RWMutex // To protect access to data
	storageDir string
}

// NewDatabase creates a new instance of Database
func NewDatabase(name string, storageDir string, ttl int64) *Database {

	db := &Database{
		data:       make(map[string]RecordHeader),
		name:       name,
		ttl:        ttl,
		stopChan:   make(chan struct{}),
		storageDir: storageDir,
	}

	if err := db.Load(); err != nil {
		log.Fatal(err)
	}

	return db
}

func (db *Database) insert(uid string, ts int64, data bytes.Buffer, isNew bool) {
	// Create the record
	record := Record{
		Timestamp: ts,
		Data:      data,
		isNew:     isNew,
	}

	// Ensure the user's data slice exists
	if _, exists := db.data[uid]; !exists {
		db.data[uid] = RecordHeader{
			hasChanged: isNew,
			records:    []Record{record},
		}
		return
	}

	records := db.data[uid]
	// Insert the record in the correct position in the slice (chronologically sorted)
	n := len(records.records)
	// Binary search for the correct insertion point
	left, right := 0, n-1
	for left <= right {
		mid := (left + right) / 2
		if records.records[mid].Timestamp < ts {
			left = mid + 1
		} else if records.records[mid].Timestamp > ts {
			right = mid - 1
		} else {
			records.records[mid] = record
			return
		}
	}

	// If not found, insert at the found index
	db.data[uid] = RecordHeader{
		hasChanged: isNew,
		records:    append(records.records[:left], append([]Record{record}, records.records[left:]...)...),
	}
}

// Insert inserts a new record for a user, maintaining chronological order
func (db *Database) Insert(uid string, ts int64, data bytes.Buffer) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.insert(uid, ts, data, true)
}

// getEarliestUserRecordIndex performs a binary search to find the index of the earliest record
// that has a timestamp >= minTimestamp. The records are assumed to be sorted by timestamp.
//
// Edge cases:
//  1. If minTimestamp is less than all records' timestamps, returns 0 (first record)
//     since all records will be >= minTimestamp
//  2. If minTimestamp is greater than all records' timestamps, returns -1 since no records
//     will be >= minTimestamp
//  3. If no records exist for the user, returns -1
//  4. If multiple records have the same timestamp >= minTimestamp, returns the leftmost/earliest one
func (db *Database) getEarliestUserRecordIndex(uid string, minTimestamp int64) int {

	if records, exists := db.data[uid]; exists {
		// Binary search for the earliest record after minTimestamp
		left, right := 0, len(records.records)-1
		var earliestIndex int
		for left <= right {
			mid := (left + right) / 2
			if records.records[mid].Timestamp >= minTimestamp {
				earliestIndex = mid
				right = mid - 1
			} else {
				left = mid + 1
			}
		}

		return earliestIndex
	}
	return -1
}

// getLatestUserRecordIndex performs a binary search to find the index of the latest record
// that has a timestamp <= maxTimestamp. The records are assumed to be sorted by timestamp.
//
// Edge cases:
//  1. If maxTimestamp is greater than all records' timestamps, returns the last record's index
//     since it will be the latest record <= maxTimestamp
//  2. If maxTimestamp is less than the first record's timestamp, returns -1 since no records
//     will be <= maxTimestamp
//  3. If no records exist for the user, returns -1
//  4. If multiple records have the same timestamp <= maxTimestamp, returns the rightmost/latest one
func (db *Database) getLatestUserRecordIndex(uid string, maxTimestamp int64) int {

	if records, exists := db.data[uid]; exists {
		// Binary search for the latest record up to maxTimestamp
		left, right := 0, len(records.records)-1
		var latestIndex int

		for left <= right {
			mid := (left + right) / 2
			if records.records[mid].Timestamp <= maxTimestamp {
				latestIndex = mid
				left = mid + 1
			} else {
				right = mid - 1
			}
		}

		return latestIndex
	}
	return -1
}

func (db *Database) GetLatestRecordForUser(uid string, maxTimestamp int64) *Record {
	db.mu.RLock()
	defer db.mu.RUnlock()
	index := db.getLatestUserRecordIndex(uid, maxTimestamp)
	if index == -1 {
		return nil
	}
	return &db.data[uid].records[index]
}

func (db *Database) GetEarliestRecordForUser(uid string, minTimestamp int64) *Record {
	db.mu.RLock()
	defer db.mu.RUnlock()
	index := db.getEarliestUserRecordIndex(uid, minTimestamp)
	if index == -1 {
		return nil
	}
	return &db.data[uid].records[index]
}

func (db *Database) GetAllLatestRecords(maxTimestamp int64) map[string]*Record {
	db.mu.RLock()
	defer db.mu.RUnlock()

	latestRecords := make(map[string]*Record)
	for uid := range db.data {
		index := db.getLatestUserRecordIndex(uid, maxTimestamp)
		if index == -1 {
			continue
		}
		latestRecords[uid] = &db.data[uid].records[index]
	}
	return latestRecords
}

// GetRecordsForUser returns all records for a given user between from and to
func (db *Database) GetRecordsForUser(uid string, from int64, to int64) []Record {
	// check that from is older than to
	if from > to {
		// return nil if timestamps are in wrong order
		log.Println("GetRecordsForUser: from is older than to", uid, from, to)
		return nil
	}

	// acquire read lock to safely access data
	db.mu.RLock()
	// defer unlock until function returns
	defer db.mu.RUnlock()

	// Check if user exists in database
	records, exists := db.data[uid]
	// return nil if user not found
	if !exists {
		// return empty slice if user not found
		return nil
	}

	// Check if there are any records
	if len(records.records) == 0 {
		// return empty slice if no records exist
		return []Record{}
	}

	// get index of earliest record after from
	startIndex := db.getEarliestUserRecordIndex(uid, from)
	// get index of latest record before to
	endIndex := db.getLatestUserRecordIndex(uid, to)

	// return nil if either index is invalid
	if startIndex == -1 || endIndex == -1 {
		// return empty slice if either index is invalid
		return []Record{}
	}

	// Check if startIndex is after endIndex
	if startIndex > endIndex {
		// return empty slice if indices are in wrong order
		return []Record{}
	}

	// Create new slice to avoid modifying original data
	result := make([]Record, endIndex-startIndex+1)
	// copy records between start and end indices
	copy(result, records.records[startIndex:endIndex+1])
	// return the copied records
	return result
}

func (db *Database) Delete(uid string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.data, uid)
	os.RemoveAll(path.Join(db.storageDir, db.name, uid))
}

// delete all records if the last timestamp is older than maxTimestamp
func (db *Database) DeleteOld() {
	db.mu.Lock()
	defer db.mu.Unlock()
	maxTimestamp := time.Now().Unix() - db.ttl*60*60
	deleted := 0
	for uid := range db.data {
		records := db.data[uid]
		// if last record is older than maxTimestamp, delete all records
		if records.records[len(records.records)-1].Timestamp < maxTimestamp {
			delete(db.data, uid)
			deleted++
		}
	}
	if deleted > 0 {
		log.Println("Deleted", deleted, "records older than", time.Unix(maxTimestamp, 0).Format("2006-01-02 15:04:05"))
	}
}

// flush to disk
func (db *Database) Flush() error {
	log.Println("Maybe flushing data to disk")

	db.mu.RLock()
	// find all records that are new
	updatedRecords := make(map[string][]Record)
	recordCount := 0
	for uid, records := range db.data {
		if !records.hasChanged {
			continue
		}
		for i, record := range records.records {
			if record.isNew {
				updatedRecords[uid] = append(updatedRecords[uid], record)
				records.records[i].isNew = false
				recordCount += 1
			}
		}
	}
	db.mu.RUnlock()
	log.Println("Found", recordCount, "new records")
	if recordCount == 0 {
		log.Println("No new records found, skipping flush")
		return nil
	}
	timestamp := time.Now().Unix()

	for uid := range updatedRecords {

		dir := path.Join(db.storageDir, db.name, uid)
		filename := path.Join(dir, fmt.Sprintf("%d.json", timestamp))
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Println("Error creating directory:", err)
			continue
		}

		jsonData, err := json.Marshal(updatedRecords[uid])
		if err != nil {
			log.Println("Error marshaling data:", err)
			continue
		}

		if err := os.WriteFile(filename, jsonData, 0644); err != nil {
			log.Println("Error writing file:", err)
			continue
		}
	}

	return nil
}

// load from disk
func (db *Database) Load() error {

	log.Println("Loading data from", db.name)
	dir := path.Join(db.storageDir, db.name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Directory doesn't exist yet, that's ok
	}

	// Get all user directories
	userDirs, err := os.ReadDir(dir)
	if err != nil {
		log.Println("Error reading directory:", err)
		return fmt.Errorf("error reading directory %s: %w", dir, err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	recordCount := 0

	// For each user directory
	for _, userDir := range userDirs {
		if !userDir.IsDir() {
			continue
		}
		uid := userDir.Name()

		// Get all JSON files in user directory
		files, err := os.ReadDir(path.Join(dir, uid))
		if err != nil {
			log.Printf("Error reading directory for user %s: %v", uid, err)
			continue
		}

		// Read and process each file
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			filePath := path.Join(dir, uid, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("Error reading file %s: %v", filePath, err)
				continue
			}

			var records []Record
			if err := json.Unmarshal(data, &records); err != nil {
				log.Printf("Error unmarshaling data from %s: %v", filePath, err)
				continue
			}

			// Insert each record
			for _, record := range records {
				db.insert(uid, record.Timestamp, record.Data, false)
				recordCount += 1
			}
		}
	}

	log.Println("Loaded", recordCount, "records from", db.name)

	return nil
}

func (db *Database) Stop() {
	if db.stopChan != nil {
		close(db.stopChan)
	}
}
