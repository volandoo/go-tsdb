# Go TSDB

A simple time series and in-memory database written in Go. It's a simple database that allows you to insert data and query it by user and collection and by timestamp.  

## Usage for development

```bash
go run main.go
```

## Usage for production

```bash
go build -o main
./main
```

## Docker

```bash
docker build -t tsdb .
docker run --rm -p 1985:1985 -v $(pwd)/.data:/app/.data tsdb
```

## API

You interact with the database using a websocket connection in port 1985.

Please refer to the example client: [example/src/index.ts](example/src/index.ts)

### Insert data

```typescript
// client is a websocket connection, refer to example/src/index.ts for more details
client.send(JSON.stringify({
    secretKey: 'your-secret-key', // the secret key is used to verify the message
    id: randomId(), // generate a random id to verify the message on socket message event
    type: 'insert', // type of the message
    data: JSON.stringify(data), // data can be anything of type string
}));
// wait for the response with the same id to verify the message
```

### Query data

```typescript
// client is a websocket connection, refer to example/src/index.ts for more details 
// ts is the timestamp to query
// collection is the collection to query
client.send(JSON.stringify({
    secretKey: 'your-secret-key', // the secret key is used to verify the message   
    id: randomId(), // generate a random id to verify the message on socket message event
    type: 'query', // type of the message
    data: JSON.stringify({ ts, collection }), // data is a json object with ts and collection
}));
// wait for the response with the same id to verify the message
```

### Query data by user

```typescript
// client is a websocket connection, refer to example/src/index.ts for more details 
// uid is the user id to query
client.send(JSON.stringify({
    secretKey: 'your-secret-key', // the secret key is used to verify the message       
    id: randomId(), // generate a random id to verify the message on socket message event
    type: 'query-user', // type of the message
    data: JSON.stringify({ uid, from, to, collection }), // data is a json object with uid, from, to and collection
}));
// wait for the response with the same id to verify the message
```

## License

MIT
