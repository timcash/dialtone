# Minimal Pure C UDP Holepunching (JS-Free Hyperswarm Alternative)

### 1. The Architecture of Bare, Hyperswarm, and Holesail

When you look at **Holesail** and the **Hyperswarm** ecosystem, the technology stack breaks down into two main layers:
1. **Peer Discovery (HyperDHT)**: Matches "topics" (public keys) to IP addresses using a Distributed Hash Table. *This is written entirely in JavaScript.*
2. **NAT Traversal & Networking (`libudx`)**: Manages the reliable UDP holepunching and stream multiplexing. *This is written in C.*

**Compiling `bare` without JavaScript:**
`bare` is compiled using `bare-make` (a CMake wrapper). If you remove the V8 JavaScript engine and JS add-ons from the build configuration, `bare` essentially ceases to be a runtime. You are left with just `libuv` (the C asynchronous event loop) and `libudx` (the underlying UDP networking protocol). 

If your goal is to have the **smallest possible pure C codebase** (removing V8, libuv, and the JS Kademlia DHT completely), you cannot use Hyperswarm's default network because there is no official pure C client for their DHT. Instead, you must implement the underlying networking concept that Hyperswarm relies on: a **UDP Holepunching** workflow using raw POSIX sockets.

---

### 2. The Minimal Pure C "Holepunch" Workflow

To create a single binary that you can send to a friend to instantly share a bidirectional UDP socket, you need three steps:
1. **Rendezvous**: Both peers ping a lightweight, known public server with their hardcoded "Topic".
2. **Discovery**: The server introduces them by swapping their public IPs and Ports.
3. **Holepunch**: Both peers simultaneously fire UDP packets at each other to pierce their respective NAT firewalls.

Here is the most simple working example in pure C.

#### `peer.c` (The Single Binary)

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <sys/socket.h>

// In Hyperswarm this is discovered via the DHT.
// For a pure C standalone binary, we use a tiny public rendezvous server.
#define RENDEZVOUS_IP "YOUR_SERVER_IP_HERE" 
#define RENDEZVOUS_PORT 7000
#define TOPIC "my-secret-mesh-topic"

int main() {
    int sock;
    struct sockaddr_in server_addr, peer_addr;
    socklen_t addr_len = sizeof(struct sockaddr_in);
    char buffer[1024];

    // 1. Create a raw UDP socket (No libuv, no V8, just pure C)
    sock = socket(AF_INET, SOCK_DGRAM, 0);

    // 2. Register Topic with the Rendezvous Server
    memset(&server_addr, 0, sizeof(server_addr));
    server_addr.sin_family = AF_INET;
    server_addr.sin_port = htons(RENDEZVOUS_PORT);
    inet_pton(AF_INET, RENDEZVOUS_IP, &server_addr.sin_addr);

    printf("Looking for peers on topic: %s\n", TOPIC);
    sendto(sock, TOPIC, strlen(TOPIC), 0, (struct sockaddr*)&server_addr, addr_len);

    // 3. Wait for the server to match us with a peer and send us their IP:Port
    int bytes = recvfrom(sock, buffer, sizeof(buffer)-1, 0, NULL, NULL);
    buffer[bytes] = '\0';
    
    char peer_ip[64];
    int peer_port;
    sscanf(buffer, "%[^:]:%d", peer_ip, &peer_port);
    printf("Peer found at %s:%d! Initiating NAT Holepunch...\n", peer_ip, peer_port);

    // 4. NAT Holepunching: Send a dummy packet to the peer's public IP
    // This tells our router/NAT to expect and allow incoming packets from them.
    memset(&peer_addr, 0, sizeof(peer_addr));
    peer_addr.sin_family = AF_INET;
    peer_addr.sin_port = htons(peer_port);
    inet_pton(AF_INET, peer_ip, &peer_addr.sin_addr);

    char *punch_msg = "PUNCH";
    sendto(sock, punch_msg, strlen(punch_msg), 0, (struct sockaddr*)&peer_addr, addr_len);

    printf("Holepunch sent. You are now in a bidirectional P2P chat!\n");

    // 5. Simple Multiplexing (Forking to handle Read/Write simultaneously)
    if (fork() == 0) {
        // Child Process: Listen for incoming P2P messages from your friend
        while(1) {
            bytes = recvfrom(sock, buffer, sizeof(buffer)-1, 0, NULL, NULL);
            if (bytes > 0 && strncmp(buffer, "PUNCH", 5) != 0) {
                buffer[bytes] = '\0';
                printf("\n[Peer]: %s", buffer);
            }
        }
    } else {
        // Parent Process: Send typed messages directly to your friend
        while(1) {
            fgets(buffer, sizeof(buffer), stdin);
            sendto(sock, buffer, strlen(buffer), 0, (struct sockaddr*)&peer_addr, addr_len);
        }
    }

    close(sock);
    return 0;
}