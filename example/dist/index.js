"use strict";
const sendMessage = async (socket, incoming) => {
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
            reject("Timeout");
        }, 5000);
        function onMessage(msg) {
            const data = JSON.parse(msg.data);
            if (data.id != incoming.id) {
                return;
            }
            socket.removeEventListener("message", onMessage);
            clearTimeout(timer);
            resolve(JSON.parse(msg.data));
        }
        function onError(event) {
            socket.removeEventListener("error", onError);
            clearTimeout(timer);
            reject(event);
        }
        socket.addEventListener("message", onMessage);
        socket.addEventListener("error", onError);
        socket.send(JSON.stringify(incoming));
    });
};
const client = new WebSocket("ws://74.220.30.94:1985/");
const secretKey = "c1132ab0-3770-4e85-932c-eae371f8dd3f";
const randomId = () => Math.random().toString(36).substring(2, 15);
const insert = async (data) => {
    const res = await sendMessage(client, {
        id: randomId(),
        secretKey,
        type: "insert",
        data: JSON.stringify(data),
    });
    return res;
};
const query = async (collection, ts) => {
    const res = await sendMessage(client, {
        id: randomId(),
        secretKey,
        type: "query",
        data: JSON.stringify({ ts, collection }),
    });
    return res;
};
const queryUser = async (uid, from, to, collection) => {
    const res = await sendMessage(client, {
        id: randomId(),
        secretKey,
        type: "query-user",
        data: JSON.stringify({ uid, from, to, collection }),
    });
    return res;
};
client.onopen = async () => {
    console.log("Connected to server");
    // insert some dummy data
    const now = Date.now() - 1000 * 10;
    const dataUserOne = [];
    for (let i = 0; i < 1000; i++) {
        dataUserOne.push({ ts: now + i, uid: "123", data: JSON.stringify({ value: i }), collection: "public" });
    }
    const dataUserTwo = [];
    for (let i = 0; i < 3000; i++) {
        dataUserTwo.push({ ts: now + i, uid: "124", data: JSON.stringify({ value: i }), collection: "public" });
    }
    const started = Date.now();
    await insert([...dataUserOne, ...dataUserTwo]);
    const duration = Date.now() - started;
    console.log(`Inserted ${dataUserOne.length + dataUserTwo.length} records in ${duration}ms`);
    // query the latest data of each user from the public collection
    const res = await query("public", Date.now());
    console.log("query public", res);
    // query the data for user 123
    const resUser = await queryUser("123", now, Date.now(), "public");
    console.log("query user 123", resUser);
};
client.onerror = (event) => {
    console.error("Error", event);
};
