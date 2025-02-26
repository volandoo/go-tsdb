package main

import (
	"fmt"
	"testing"
)

func createRecords(database *Database, uid string, count int) []Record {
	records := make([]Record, count)
	for i := 0; i < count; i++ {
		database.Insert(uid, int64(i+1), fmt.Sprintf("test_%d", i+1))
	}
	return records
}

func TestGetLatest(t *testing.T) {
	db := NewDatabase("test", "test", 1)
	defer db.Stop()

	createRecords(db, "1", 15)
	createRecords(db, "2", 12)
	createRecords(db, "3", 9)

	records := db.GetAllLatestRecords(15)
	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}

	// expect records["1"].Data = "test_15"
	if data := (records["1"].Data); data != "test_15" {
		t.Errorf("Expected test_15, got %s", data)
	}
	if data := (records["2"].Data); data != "test_12" {
		t.Errorf("Expected test_12, got %s", data)
	}
	if data := (records["3"].Data); data != "test_9" {
		t.Errorf("Expected test_9, got %s", data)
	}

}

func TestGetLatestUpTo(t *testing.T) {
	db := NewDatabase("test", "test", 1)
	defer db.Stop()

	createRecords(db, "1", 15)
	createRecords(db, "2", 12)
	createRecords(db, "3", 9)

	records := db.GetAllLatestRecords(3)
	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}

	// expect records["1"].Data = "test_3"
	if data := (records["1"].Data); data != "test_3" {
		t.Errorf("Expected test_3, got %s", data)
	}
	if data := (records["2"].Data); data != "test_3" {
		t.Errorf("Expected test_3, got %s", data)
	}
	if data := (records["3"].Data); data != "test_3" {
		t.Errorf("Expected test_3, got %s", data)
	}
}

func TestGetRecordsForUserRange(t *testing.T) {
	db := NewDatabase("test", "test", 1)
	defer db.Stop()

	createRecords(db, "1", 1500)

	records := db.GetRecordsForUser("1", 160, 180)
	if len(records) != (180 - 160 + 1) {
		t.Errorf("Expected %d records, got %d", 180-160+1, len(records))
	}

	// expect records[0].Data = "test_160"
	if data := (records[0].Data); data != "test_160" {
		t.Errorf("Expected test_160, got %s", data)
	}

	// expect records[1].Data = "test_161"
	if data := (records[1].Data); data != "test_161" {
		t.Errorf("Expected test_161, got %s", data)
	}

	// expect records[19].Data = "test_179"
	if data := (records[19].Data); data != "test_179" {
		t.Errorf("Expected test_179, got %s", data)
	}

	// expect records[20].Data = "test_180"
	if data := (records[20].Data); data != "test_180" {
		t.Errorf("Expected test_180, got %s", data)
	}
}

func TestGetRecordsForUserOutOfRangeHigh(t *testing.T) {
	db := NewDatabase("test", "test", 1)
	defer db.Stop()

	createRecords(db, "1", 1500)

	records := db.GetRecordsForUser("1", 1501, 1502)
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}
}

func TestGetRecordsForUserOutOfRangeLow(t *testing.T) {
	db := NewDatabase("test", "test", 1)
	defer db.Stop()

	createRecords(db, "1", 1500)

	records := db.GetRecordsForUser("1", -10, -1)
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}
}

func TestGetRecordsForUserOutOfRangeHighAndLow(t *testing.T) {
	db := NewDatabase("test", "test", 1)
	defer db.Stop()

	createRecords(db, "1", 1500)

	records := db.GetRecordsForUser("1", -10, 1502)
	if len(records) != 1500 {
		t.Errorf("Expected 1500 records, got %d", len(records))
	}
}

func TestGetLatestRecordUpTo(t *testing.T) {
	db := NewDatabase("test", "test", 1)
	defer db.Stop()

	createRecords(db, "1", 1500)

	record := db.GetLatestRecordForUser("1", 1500)
	if record.Data != "test_1500" {
		t.Errorf("Expected test_1500, got %s", record.Data)
	}

	record = db.GetLatestRecordForUser("1", 1501)
	if record.Data != "test_1500" {
		t.Errorf("Expected test_1500, got %s", record.Data)
	}

	record = db.GetLatestRecordForUser("1", 1499)
	if record.Data != "test_1499" {
		t.Errorf("Expected test_1499, got %s", record.Data)
	}

}
