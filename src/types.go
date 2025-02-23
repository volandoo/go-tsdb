package main

import "github.com/gorilla/websocket"

// all requests have an id, secret key, message type and data
type request struct {
	Id          *string `json:"id"`
	MessageType *string `json:"type"`
	Data        *string `json:"data"`
}

// data payload for insert requests
type dataPayload struct {
	Ts         *int64  `json:"ts"`
	Uid        *string `json:"uid"`
	Data       *string `json:"data"`
	Collection *string `json:"collection"`
}

type dataPayloadResponse struct {
	Id string `json:"id"`
}

// query requests have a timestamp and collection
type query struct {
	Ts         *int64  `json:"ts"`
	Collection *string `json:"collection"`
}

// query responses have a list of records
type queryResponse struct {
	Id      string             `json:"id"`
	Records map[string]*Record `json:"records"`
}

type queryUser struct {
	Uid        *string `json:"uid"`
	From       *int64  `json:"from"`
	To         *int64  `json:"to"`
	Collection *string `json:"collection"`
}

// query user responses have a list of records
type queryUserResponse struct {
	Id      string   `json:"id"`
	Records []Record `json:"records"`
}

type queryDeleteUser struct {
	Uid        *string `json:"uid"`
	Collection *string `json:"collection"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type callback func(message request) ([]byte, error)
