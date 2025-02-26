package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Collection struct {
	Name string
	TTL  int
}

// Two cases:
// 1. collection:ttl
// 2. collection.*:ttl
// error if missing :ttl or more than one .*
func NewCollection(name string) Collection {
	parts := strings.Split(name, ":")
	if len(parts) != 2 {
		// panic
		panic("invalid collection name: " + name)
	}
	// check if there is more than one .*
	if strings.Count(parts[0], ".") > 1 {
		// panic
		panic("invalid collection name: " + name)
	}
	ttl, err := strconv.Atoi(parts[1])
	if err != nil {
		// panic
		panic("invalid collection name: " + name)
	}
	if ttl < 0 {
		// panic
		panic("invalid collection name: " + name)
	}
	return Collection{
		Name: parts[0],
		TTL:  ttl,
	}
}

func (c Collection) IsCollection(other string) bool {
	parts1 := strings.Split(other, ".")
	parts2 := strings.Split(c.Name, ".")
	if len(parts1) != len(parts2) {
		return false
	}
	return parts1[0] == parts2[0]
}

func setupDatabases(storageDir string, collections []Collection) map[string]*Database {
	databases := make(map[string]*Database)

	if storageDir == "" {
		log.Println("Storage directory is not set, data will not be stored on disk")
		return databases
	}

	log.Println("Setting up databases")
	// check if .data directory exists
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		os.Mkdir(storageDir, 0755)
	}
	collectionDirs, err := os.ReadDir(storageDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, collectionDir := range collectionDirs {
		if !collectionDir.IsDir() {
			continue
		}

		// check if the collection is in the collections array
		found := false
		for _, collection := range collections {
			if collection.IsCollection(collectionDir.Name()) {
				databases[collectionDir.Name()] = NewDatabase(collectionDir.Name(), storageDir, int64(collection.TTL))
				found = true
				break
			}
		}
		if !found {
			continue
		}
	}
	return databases
}

func startServer(secretKey string, colls []string, storageDir string, storageInterval int) error {

	collections := make([]Collection, len(colls))
	for i, coll := range colls {
		collections[i] = NewCollection(coll)
	}

	databases := setupDatabases(storageDir, collections)
	for _, db := range databases {
		defer db.Stop()
	}

	onWebSocketMessage := func(w http.ResponseWriter, r *http.Request, callback callback) {
		log.Println("WebSocket connection received")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade failed:", err)
			return
		}
		defer conn.Close()
		var apiKey string
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("Error reading message:", err)
				break
			}

			var req request
			if err := json.Unmarshal(msg, &req); err != nil {
				log.Println("Error processing message:", err)
				break
			}
			var resp []byte
			if *req.MessageType == "api-key" {
				apiKey = *req.Data
				if apiKey != secretKey {
					log.Println("Invalid API key")
					break
				}
				resp, err = json.Marshal(dataPayloadResponse{Id: *req.Id})
				if err != nil {
					log.Println("Error processing message:", err)
					break
				}
			} else {
				if apiKey != secretKey {
					log.Println("API key is required")
					break
				}
				resp, err = callback(req)
				if err != nil {
					log.Println("Error processing message:", err)
					break
				}
			}
			conn.WriteMessage(websocket.TextMessage, resp)
			log.Println("Sent response", len(resp))
		}
		log.Println("WebSocket connection closed")
	}

	handleInsert := func(id string, message []byte) ([]byte, error) {
		var messages []dataPayload
		if err := json.Unmarshal(message, &messages); err != nil {
			return nil, err
		}
		for _, msg := range messages {
			// all these are required!
			if msg.Ts == nil || msg.Uid == nil || msg.Data == nil || msg.Collection == nil {
				return nil, errors.New("invalid message")
			}
			// check if the collection is already in the databases
			if db := databases[*msg.Collection]; db != nil {
				db.Insert(*msg.Uid, *msg.Ts, *msg.Data)
			} else {
				found := false
				// check if the collection is not in the databases, yet
				for _, collection := range collections {
					if collection.IsCollection(*msg.Collection) {
						databases[*msg.Collection] = NewDatabase(*msg.Collection, storageDir, int64(collection.TTL))
						found = true
						break
					}
				}
				if !found {
					return nil, fmt.Errorf("collection %s not found", *msg.Collection)
				}
			}
		}
		return json.Marshal(dataPayloadResponse{Id: id})
	}

	handleQuery := func(id string, message []byte) ([]byte, error) {
		var queryMessage query
		if err := json.Unmarshal(message, &queryMessage); err != nil {
			return nil, err
		}
		if queryMessage.Ts == nil {
			return nil, errors.New("ts is required")
		}
		if queryMessage.Collection == nil {
			return nil, errors.New("collection is required")
		}
		if db := databases[*queryMessage.Collection]; db != nil {
			var response map[string]*Record
			if queryMessage.Uid != "" {
				response = map[string]*Record{
					queryMessage.Uid: db.GetLatestRecordForUser(queryMessage.Uid, *queryMessage.Ts),
				}
			} else {
				response = db.GetAllLatestRecords(*queryMessage.Ts)
			}
			return json.Marshal(queryResponse{Id: id, Records: response})
		}
		return json.Marshal(queryResponse{Id: id, Records: map[string]*Record{}})
	}

	handleQueryUser := func(id string, message []byte) ([]byte, error) {
		var queryUserMessage queryUser
		if err := json.Unmarshal(message, &queryUserMessage); err != nil {
			return nil, err
		}
		if queryUserMessage.Uid == nil {
			return nil, errors.New("uid is required")
		}
		if queryUserMessage.From == nil {
			return nil, errors.New("from is required")
		}
		if queryUserMessage.To == nil {
			return nil, errors.New("to is required")
		}
		if queryUserMessage.Collection == nil {
			return nil, errors.New("collection is required")
		}
		if db := databases[*queryUserMessage.Collection]; db != nil {
			response := db.GetRecordsForUser(*queryUserMessage.Uid, *queryUserMessage.From, *queryUserMessage.To)
			return json.Marshal(queryUserResponse{Id: id, Records: response})
		}
		return json.Marshal(queryUserResponse{Id: id, Records: []Record{}})
	}

	handleDeleteUser := func(id string, message []byte) ([]byte, error) {
		var queryMessage queryDeleteUser
		if err := json.Unmarshal(message, &queryMessage); err != nil {
			return nil, err
		}
		if queryMessage.Uid == nil {
			return nil, errors.New("uid is required")
		}
		if queryMessage.Collection == "" {
			for _, db := range databases {
				db.Delete(*queryMessage.Uid)
			}
			return json.Marshal(dataPayloadResponse{Id: id})
		}
		if db := databases[queryMessage.Collection]; db != nil {
			db.Delete(*queryMessage.Uid)
			return json.Marshal(dataPayloadResponse{Id: id})
		}
		return json.Marshal(dataPayloadResponse{Id: id})
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not found"))
			return
		}
		onWebSocketMessage(w, r, func(message request) ([]byte, error) {
			if *message.MessageType == "insert" {
				return handleInsert(*message.Id, []byte(*message.Data))
			}
			if *message.MessageType == "query" {
				return handleQuery(*message.Id, []byte(*message.Data))
			}
			if *message.MessageType == "query-user" {
				return handleQueryUser(*message.Id, []byte(*message.Data))
			}
			if *message.MessageType == "delete-user" {
				return handleDeleteUser(*message.Id, []byte(*message.Data))
			}
			return nil, errors.New("invalid message type")
		})
	})

	if storageInterval > 0 {
		// timer to flush to disk every one minute
		go func() {
			for {
				log.Println("Flushing data to disk")
				for _, db := range databases {
					db.DeleteOld()
					db.Flush()
				}

				<-time.After(time.Duration(storageInterval) * time.Second)
			}
		}()
	}
	log.Println("Listening on port 1985")
	return http.ListenAndServe("0.0.0.0:1985", nil)
}
