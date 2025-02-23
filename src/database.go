package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type Record struct {
	Timestamp int64  `json:"ts"`
	Data      string `json:"data"`
}

type UserData struct {
	Records []Record     `json:"records"`
	mu      sync.RWMutex // To protect concurrent access to Records
}

type Database struct {
	data       map[string]*UserData
	dataLength int
	name       string
	ttl        int64 // hours
	stopChan   chan struct{}
	mu         sync.RWMutex // To protect access to data
}

// NewDatabase creates a new instance of Database
func NewDatabase(name string, ttl int64) *Database {

	db := &Database{
		data: make(map[string]*UserData),
		name: name,
		ttl:  ttl,
	}

	if err := db.Load(); err != nil {
		log.Fatal(err)
	}
	// Goroutine for periodic flushing
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				db.DeleteOld()
				db.Flush()
			case <-db.stopChan:
				log.Println("Stopping database flush goroutine")
				return
			}
		}
	}()

	return db
}

// Insert inserts a new record for a user, maintaining chronological order
func (db *Database) Insert(uid string, ts int64, data string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Create the record
	record := Record{Timestamp: ts, Data: data}

	// Ensure the user's data slice exists
	if _, exists := db.data[uid]; !exists {
		db.data[uid] = &UserData{}
	}

	userData := db.data[uid]
	userData.mu.Lock()
	defer userData.mu.Unlock()

	// Insert the record in the correct position in the slice (chronologically sorted)
	records := userData.Records
	n := len(records)

	// If no records, just append
	if n == 0 {
		userData.Records = append(records, record)
		return
	}

	// Binary search for the correct insertion point
	left, right := 0, n-1
	for left <= right {
		mid := (left + right) / 2
		if records[mid].Timestamp < ts {
			left = mid + 1
		} else if records[mid].Timestamp > ts {
			right = mid - 1
		} else {
			records[mid] = record
			return
		}
	}

	// If not found, insert at the found index
	userData.Records = append(records[:left], append([]Record{record}, records[left:]...)...)
}

// getLatestRecordUpTo retrieves the latest record up to a given timestamp for a user
func (db *Database) getLatestRecordUpTo(uid string, maxTimestamp int64) *Record {

	if userData, exists := db.data[uid]; exists {
		userData.mu.RLock()
		defer userData.mu.RUnlock()

		// Binary search for the latest record up to maxTimestamp
		records := userData.Records
		left, right := 0, len(records)-1
		var latest *Record

		for left <= right {
			mid := (left + right) / 2
			if records[mid].Timestamp <= maxTimestamp {
				latest = &records[mid]
				left = mid + 1
			} else {
				right = mid - 1
			}
		}

		return latest
	}
	return nil
}

// GetAllLatestRecordsUpTo retrieves the latest records for all users up to a given timestamp
func (db *Database) GetAllLatestRecordsUpTo(maxTimestamp int64) map[string]*Record {
	db.mu.RLock()
	defer db.mu.RUnlock()

	latestRecords := make(map[string]*Record)

	for uid := range db.data {
		record := db.getLatestRecordUpTo(uid, maxTimestamp)
		if record != nil {
			latestRecords[uid] = record
		}
	}

	return latestRecords
}

// get records for a user from timestamp to timestamp
func (db *Database) GetRecordsForUser(uid string, fromTimestamp int64, toTimestamp int64) []Record {

	// check that fromTimestamp is older than toTimestamp
	if fromTimestamp > toTimestamp {
		return nil
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	// Check if user exists
	userData, exists := db.data[uid]
	if !exists || len(userData.Records) == 0 {
		return nil
	}

	userData.mu.RLock()
	defer userData.mu.RUnlock()

	records := userData.Records
	n := len(records)

	// Binary search for the first record >= fromTimestamp
	left, right := 0, n-1
	firstIndex := n // Default to out-of-bounds index

	for left <= right {
		mid := (left + right) / 2
		if records[mid].Timestamp < fromTimestamp {
			left = mid + 1
		} else {
			firstIndex = mid
			right = mid - 1
		}
	}

	// Binary search for the last record <= toTimestamp
	left, right = firstIndex, n-1
	lastIndex := -1 // Default to out-of-bounds index

	for left <= right {
		mid := (left + right) / 2
		if records[mid].Timestamp > toTimestamp {
			right = mid - 1
		} else {
			lastIndex = mid
			left = mid + 1
		}
	}

	// Return safely, avoiding out-of-bounds issues
	if firstIndex >= n || lastIndex < 0 || firstIndex > lastIndex {
		return nil
	}
	return records[firstIndex : lastIndex+1]
}

func (db *Database) Delete(uid string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.data, uid)
}

// delete all records if the last timestamp is older than maxTimestamp
func (db *Database) DeleteOld() {
	db.mu.Lock()
	defer db.mu.Unlock()
	maxTimestamp := time.Now().Unix() - db.ttl*60*60
	deleted := 0
	for uid := range db.data {
		userData := db.data[uid]
		// if last record is older than maxTimestamp, delete all records
		if userData.Records[len(userData.Records)-1].Timestamp < maxTimestamp {
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

	db.mu.RLock()
	defer db.mu.RUnlock()

	// write to disk using db.name as the filename .json
	data, err := json.Marshal(db.data)
	if err != nil {
		return err
	}
	if db.dataLength != len(data) {
		db.dataLength = len(data)
		log.Println("Flushing data to disk", db.dataLength)
		// check if data directory exists
		if _, err := os.Stat(".data"); os.IsNotExist(err) {
			os.Mkdir(".data", 0755)
		}
		err = os.WriteFile(".data/"+db.name+".json", data, 0644)
		if err != nil {
			log.Println("Error flushing data to disk", err)
		}
		return err
	}
	log.Println("No data to flush")
	return nil
}

// load from disk
func (db *Database) Load() error {
	// check if data directory exists
	if _, err := os.Stat(".data"); os.IsNotExist(err) {
		os.Mkdir(".data", 0755)
	}

	// check if file exists
	if _, err := os.Stat(".data/" + db.name + ".json"); os.IsNotExist(err) {
		return nil
	}

	log.Println("Loading", db.name, "...")
	data, err := os.ReadFile(".data/" + db.name + ".json")
	if err != nil {
		return err
	}
	log.Println("Unmarshalling", db.name, "...")
	e := json.Unmarshal(data, &db.data)
	if e != nil {
		return e
	}
	db.dataLength = len(data)
	log.Println("Loaded", db.dataLength, "records from disk")
	return nil
}

func (db *Database) Stop() {
	if db.stopChan != nil {
		close(db.stopChan)
	}
}
