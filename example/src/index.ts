type Message = {
    id: string;
    [key: string]: any;
};

const sendMessage = async <T>(socket: WebSocket, incoming: Message) => {
    return new Promise<T>((resolve, reject) => {
        const timer = setTimeout(() => {
            reject("Timeout");
        }, 5000);
        function onMessage(msg: MessageEvent) {
            const data = JSON.parse(msg.data);
            if (data.id != incoming.id) {
                return;
            }
            socket.removeEventListener("message", onMessage);
            clearTimeout(timer);
            resolve(JSON.parse(msg.data) as T);
        }
        function onError(event: Event) {
            socket.removeEventListener("error", onError);
            clearTimeout(timer);
            reject(event);
        }
        socket.addEventListener("message", onMessage);
        socket.addEventListener("error", onError);
        socket.send(JSON.stringify(incoming));
    });
};

const client = new WebSocket("ws://localhost:1985/");

type TSDBRecord = {
    ts: number;
    data: string;
};

type TSDBRequest = {
    id: string;
    secretKey: string;
    type: "insert" | "query" | "query-user";
    data: string; // JSON string of type TSDBUserMessage or TSDBQueryMessage or TSDBQueryUserMessage
};

type TSDBInsertMessageRequest = {
    ts: number;
    uid: string;
    data: string;
    collection: string;
};

type TSDBInsertMessageResponse = {
    id: string;
};

type TSDBQueryMessageRequest = {
    ts: number;
    collection: string;
};

type TSDBQueryMessageResponse = {
    id: string;
    records: { [uid: string]: TSDBRecord };
};

type TSDBQueryUserMessage = {
    uid: string;
    from: number;
    to: number;
    collection: string;
};

type TSDBQueryUserMessageResponse = {
    id: string;
    records: TSDBRecord[];
};

const secretKey = process.env.SECRET_KEY;
const randomId = () => Math.random().toString(36).substring(2, 15);
const insert = async (data: TSDBInsertMessageRequest[]) => {
    const res = await sendMessage<TSDBInsertMessageResponse>(client, {
        id: randomId(),
        secretKey,
        type: "insert",
        data: JSON.stringify(data),
    });
    return res;
};
const query = async (collection: string, ts: number) => {
    const res = await sendMessage<TSDBQueryMessageResponse>(client, {
        id: randomId(),
        secretKey,
        type: "query",
        data: JSON.stringify({ ts, collection }),
    });
    return res;
};
const queryUser = async (uid: string, from: number, to: number, collection: string) => {
    const res = await sendMessage<TSDBQueryUserMessageResponse>(client, {
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
    const dataUserOne: TSDBInsertMessageRequest[] = [];
    for (let i = 0; i < 1000; i++) {
        dataUserOne.push({ ts: now + i, uid: "123", data: JSON.stringify({ value: i }), collection: "public" });
    }
    const dataUserTwo: TSDBInsertMessageRequest[] = [];
    for (let i = 0; i < 3000; i++) {
        dataUserTwo.push({ ts: now + i, uid: "124", data: JSON.stringify({ value: i }), collection: "public" });
    }

    await insert([...dataUserOne, ...dataUserTwo]);

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
