package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func setupDatabases() map[string]*Database {
	databases := make(map[string]*Database)

	log.Println("Setting up databases")
	// check if .data directory exists
	if _, err := os.Stat(".data"); os.IsNotExist(err) {
		os.Mkdir(".data", 0755)
	}

	files, err := filepath.Glob(".data/*.json")
	log.Println("Found", len(files), "databases")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {

		// file without ./data and .json
		dbName := strings.TrimPrefix(file, ".data/")
		dbName = strings.TrimSuffix(dbName, ".json")
		databases[dbName] = NewDatabase(dbName, 12)
		log.Println("Loaded", dbName)
	}
	return databases
}

func main() {

	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		log.Fatal("SECRET_KEY is not set")
	}

	databases := setupDatabases()
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

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Error reading message:", err)
				break
			}
			started := time.Now()
			response, err := callback(message)
			if err != nil {
				log.Println("Error processing message:", err)
				break
			}
			conn.WriteMessage(websocket.TextMessage, response)
			elapsed := time.Since(started)
			log.Println("Processed message in", elapsed)
		}
	}

	handleInsert := func(id string, message []byte) ([]byte, error) {
		var messages []dataPayload
		if err := json.Unmarshal(message, &messages); err != nil {
			return nil, err
		}
		for _, msg := range messages {
			if msg.Ts == nil || msg.Uid == nil || msg.Data == nil || msg.Collection == nil {
				return nil, errors.New("invalid message")
			}
			if *msg.Collection == "public" || *msg.Collection == "private" || strings.HasPrefix(*msg.Collection, "event.") || strings.HasPrefix(*msg.Collection, "group.") {
				if _, ok := databases[*msg.Collection]; !ok {
					databases[*msg.Collection] = NewDatabase(*msg.Collection, 12)
				}
				databases[*msg.Collection].Insert(*msg.Uid, *msg.Ts, *msg.Data)
			} else {
				return nil, errors.New("invalid collection")
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
			response := db.GetAllLatestRecordsUpTo(*queryMessage.Ts)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("1 WebSocket connection received", r.URL.Path)
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not found"))
			return
		}
		log.Println("2 WebSocket connection received", r.URL.Path)
		onWebSocketMessage(w, r, func(msg []byte) ([]byte, error) {
			var message request
			if err := json.Unmarshal(msg, &message); err != nil {
				return nil, err
			}
			if *message.SecretKey != secretKey {
				return nil, errors.New("invalid secret key")
			}
			if *message.MessageType == "insert" {
				return handleInsert(*message.Id, []byte(*message.Data))
			}
			if *message.MessageType == "query" {
				return handleQuery(*message.Id, []byte(*message.Data))
			}
			if *message.MessageType == "query-user" {
				return handleQueryUser(*message.Id, []byte(*message.Data))
			}
			return nil, errors.New("invalid message type")
		})
	})

	log.Println("Listening on port 1985")
	if err := http.ListenAndServe("0.0.0.0:1985", nil); err != nil {
		log.Fatal(err)
	}
}
