import { serve } from "bun";
import { join } from "path";
import { connect, JSONCodec } from "nats";

const PORT = 3002;
const NATS_URL = "nats://localhost:4222";
const jc = JSONCodec();

// NATS Connection and subscription
let natsConn: any = null;
const clients = new Set<any>();

async function setupNats() {
    try {
        console.log(`Connecting to NATS at ${NATS_URL}...`);
        natsConn = await connect({ servers: NATS_URL });
        console.log("Connected to NATS");

        const sub = natsConn.subscribe("rover.state");
        (async () => {
            for await (const msg of sub) {
                const data = jc.decode(msg.data);
                // Broadcast to all connected WebSocket clients
                const message = JSON.stringify(data);
                for (const client of clients) {
                    client.send(message);
                }
            }
        })();
    } catch (err) {
        console.error("Error connecting to NATS:", err);
        setTimeout(setupNats, 5000);
    }
}

setupNats();

serve({
    port: PORT,
    fetch(req, server) {
        const url = new URL(req.url);

        // Handle WebSocket upgrade
        if (url.pathname === "/telemetry-ws") {
            if (server.upgrade(req)) {
                return;
            }
        }

        let path = url.pathname;
        if (path === "/") path = "/index.html";

        const filePath = join(import.meta.dir, path);
        const file = Bun.file(filePath);

        if (path.endsWith(".ts")) {
            return new Response(file, {
                headers: { "Content-Type": "application/javascript" }
            });
        }

        return new Response(file);
    },
    websocket: {
        open(ws) {
            console.log("WebSocket client connected");
            clients.add(ws);
        },
        message(ws, message) {
            console.log(`Received message: ${message}`);
            // Handle commands from UI if needed
            if (natsConn) {
                natsConn.publish("rover.command", message);
            }
        },
        close(ws) {
            console.log("WebSocket client disconnected");
            clients.delete(ws);
        },
    },
});

console.log(`Web server running at http://localhost:${PORT}`);
