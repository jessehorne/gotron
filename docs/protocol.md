# Armagetron Advanced Network Protocol Specification

**Version:** Based on Armagetron Advanced main branch (0.4.x)

**Author:** Jesse + Claude (ai bad)

## Table of Contents

1. [Overview](#overview)
2. [Transport Layer](#transport-layer)
3. [Message Format](#message-format)
4. [Data Types](#data-types)
5. [Server Discovery](#server-discovery)
6. [Connection Handshake](#connection-handshake)
7. [Game Messages](#game-messages)
8. [Message Catalog](#message-catalog)
9. [Implementation Guide](#implementation-guide)
10. [Advanced Topics](#advanced-topics)

---

## 1. Overview

Armagetron Advanced uses a custom UDP-based protocol for both server discovery and gameplay. The protocol supports two message formats:

- **Legacy Stream Format**: Used by older versions (< 0.2.9.0.0), based on binary serialization
- **Protocol Buffers Format**: Used by modern versions (>= 0.2.9.0.0), based on Google Protocol Buffers

Modern clients and servers support both formats for backward compatibility.

### Key Characteristics

- **Transport:** UDP (unreliable, connectionless)
- **Port:** Default 4534 (configurable)
- **Encoding:** Protocol Buffers (preferred) or legacy binary streams
- **Byte Order:** Little-endian for multi-byte values in protobuf, big-endian for message headers
- **Message ID Flag:** High bit (0x8000) indicates protobuf format

---

## 2. Transport Layer

### 2.1 UDP Protocol

All communication uses UDP datagrams. There is no TCP connection.

- **Default Port:** 4534
- **Broadcast Discovery:** UDP broadcast to 255.255.255.255:4534
- **No Connection State:** Each message is self-contained

### 2.2 Network Topology

```
Client                                  Server
  |                                       |
  |-------- UDP Broadcast (port 4534) -->|  (Server Discovery)
  |<------- Server Info Response --------|
  |                                       |
  |-------- Login Request -------------->|  (Connection)
  |<------- Login Accepted --------------|
  |                                       |
  |<-------- Game State Updates -------->|  (Gameplay)
  |-------- Player Input ---------------->|
```

---

## 3. Message Format

### 3.1 Message Structure

Every message consists of:

```
+----------------------+
| Message ID (2 bytes) |  Big-endian uint16
+----------------------+
| Payload (N bytes)    |  Format depends on message type
+----------------------+
```

**Message ID Format:**
- Bits 0-14: Actual message ID (0-16383)
- Bit 15: Protobuf flag (0 = legacy, 1 = protobuf)

```
Message ID: 0x8032 = 0b1000000000110010
            ^         ^
            |         └─ Message ID: 50 (SmallServerInfo)
            └─────────── Protobuf flag: 1
```

### 3.2 Encoding Format Detection

```c
// Extract message ID and format
uint16_t rawID = (data[0] << 8) | data[1];  // Big-endian
bool isProtoBuf = (rawID & 0x8000) != 0;
uint16_t messageID = rawID & 0x7FFF;
```

### 3.3 Protocol Buffer Encoding

Protocol Buffer messages use standard proto2 encoding with one special addition:

**Legacy Message End Marker:** Field 20000 (bool, always true) marks the end of compatible fields. Extensions can be added after this marker without breaking compatibility with older clients.

Example protobuf structure:
```protobuf
message SmallServerInfo {
    optional SmallServerInfoBase base = 1;
    optional int32 transaction = 2;
    optional bool legacy_message_end_marker = 20000;
    // Extensions can go here without breaking old clients
}
```

---

## 4. Data Types

### 4.1 Protocol Buffer Data Types

Standard protobuf encoding is used:

| Proto Type | Wire Type | Encoding |
|------------|-----------|----------|
| int32 | 0 (varint) | Signed zigzag encoding |
| uint32 | 0 (varint) | Unsigned varint |
| sint32 | 0 (varint) | Signed zigzag encoding |
| fixed32 | 5 (32-bit) | Little-endian 32-bit |
| float | 5 (32-bit) | IEEE 754 little-endian |
| string | 2 (length-delimited) | Length prefix + UTF-8 data |
| bool | 0 (varint) | 0 = false, 1 = true |

### 4.2 Protocol Buffer Wire Format

**Varint Encoding Example:**
```
Value 4534:
Binary: 10001101100110
Varint: B6 23
  = 0xB6 (10110110) | 0x23 (00100011)
  = (0x36 | 0x80) then (0x23)
  = continue bit set, then final byte
```

### 4.3 Legacy Stream Format Data Types

In legacy format (not recommended for new implementations), data is serialized as sequences of unsigned shorts (16-bit values):

#### Integer (int32)
```
[lower 16 bits (unsigned)] [upper 16 bits (signed)]
```

Example for value 4534:
```
Bytes: B6 11 00 00
       └──┬─┘ └──┬─┘
       0x11B6   0x0000
       = 4534   = 0
Result: 4534 + (0 << 16) = 4534
```

#### String (tString)
```
[length (ushort)] [char pairs as shorts] [optional last char]
```

- Length includes null terminator
- Characters packed 2 per short (little-endian)
- Odd-length strings have final char in low byte of last short

Example for "Test":
```
Length: 5 (4 chars + null) = 05 00
Data:   'T''e' = 54 65
        's''t' = 73 74
        '\0'   = 00 00
Result: 05 00 54 65 73 74 00 00
```

---

## 5. Server Discovery

Server discovery uses LAN broadcast or master server queries.

### 5.1 LAN Discovery Flow

```
1. Client broadcasts RequestSmallServerInfo to 255.255.255.255:4534
2. Servers respond with SmallServerInfo
3. Client optionally requests BigServerInfo for full details
4. Server responds with BigServerInfo
```

### 5.2 Message: RequestSmallServerInfo (ID 52)

**Direction:** Client → Server (broadcast)  
**Format:** Protobuf

```protobuf
message RequestSmallServerInfo {
    optional int32 transaction = 1;  // Transaction tracking
    optional bool legacy_message_end_marker = 20000;
}
```

**Binary Example (protobuf):**
```
Message Header: 80 34  (0x8034 = protobuf flag + ID 52)
Payload:        08 00  (field 1: varint 0 for transaction)
                A0 9C 02 01  (field 20000: bool true)
```

### 5.3 Message: SmallServerInfo (ID 50)

**Direction:** Server → Client  
**Format:** Protobuf

```protobuf
message SmallServerInfoBase {
    optional int32  port = 1;
    optional string hostname = 2;
    optional bool legacy_message_end_marker = 20000;
}

message SmallServerInfo {
    optional SmallServerInfoBase base = 1;
    optional int32 transaction = 2;
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Provides minimal server information for browser listing.

**Binary Example (protobuf):**
```
Message Header: 80 32  (0x8032 = protobuf flag + ID 50)
Payload:
  0A 07          (field 1: length 7 for embedded message)
    08 B6 23     (base.port field 1: varint 4534)
    A0 9C 02 01  (base.legacy_marker field 20000)
  10 00          (transaction field 2: varint 0)
  A0 9C 02 01    (legacy_marker field 20000)
```

### 5.4 Message: RequestBigServerInfo (ID 53)

**Direction:** Client → Server  
**Format:** Protobuf

```protobuf
message RequestBigServerInfo {
    optional bool legacy_message_end_marker = 20000;
}
```

Usually sent after receiving SmallServerInfo to get full details.

### 5.5 Message: BigServerInfo (ID 51)

**Direction:** Server → Client  
**Format:** Protobuf

```protobuf
message BigServerInfo {
    optional SmallServerInfoBase base = 1;
    optional string name = 2;        // Server name
    optional int32 users = 3;        // Current player count
    optional VersionSync version = 4; // Protocol version
    optional string release = 5;     // Version string (e.g., "0.4.0")
    optional int32 max_users = 6;    // Max players
    optional string usernames = 7;   // Newline-separated player names
    optional string options = 8;     // Server settings description
    optional string url = 9;         // Server website
    optional string global_ids = 10; // Global player IDs
    optional SettingsDigest settings = 11; // Game settings
    optional bool legacy_message_end_marker = 20000;
}

message SettingsDigest {
    optional uint32 flags = 1;
    optional int32 min_play_time_total = 2;
    optional int32 min_play_time_online = 3;
    optional int32 min_play_time_team = 4;
    optional float cycle_delay = 5;
    optional float acceleration = 6;
    optional float rubber_wall_hump = 7;
    optional float rubber_hit_wall_ratio = 8;
    optional float walls_length = 9;
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Provides complete server information including player list and settings.

---

## 6. Connection Handshake

### 6.1 Login Flow

```
Client                                Server
  |                                     |
  |------ Login (ID 6) ---------------->|
  |                                     |
  |<----- LoginAccepted (ID 5) ---------|
  |  OR                                 |
  |<----- LoginDenied (ID 3) -----------|
  |  OR                                 |
  |<----- LoginIgnored (ID 4) ----------|
```

### 6.2 Message: Login (ID 6)

**Direction:** Client → Server  
**Format:** Protobuf

```protobuf
message Login {
    optional uint32 rate = 1;          // Bandwidth (kbyte/s)
    optional string big_brother = 2;   // Hardware stats
    optional VersionSync version = 3;  // Client version
    optional string authentication_methods = 4;
    optional Hash token = 5;           // Security token
    optional EncodingOptions options = 6;
    optional bool legacy_message_end_marker = 20000;
}

message VersionSync {
    optional int32 min = 1;  // Min protocol version supported
    optional int32 max = 2;  // Max protocol version supported
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Client requests to connect to server.

**Version Negotiation:** The server must support a version in the range [client.min, client.max] AND the client must support a version in the range [server.min, server.max]. The overlapping range is used.

### 6.3 Message: LoginAccepted (ID 5)

**Direction:** Server → Client  
**Format:** Protobuf

```protobuf
message LoginAccepted {
    optional uint32 net_id = 1;       // Client's network ID
    optional VersionSync version = 2; // Negotiated version
    optional string address = 3;      // Client IP as seen by server
    optional Hash token = 4;          // Echo of client token
    optional EncodingOptions options = 5;
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Server accepts connection and assigns network ID.

**Client Network ID:** Used to identify this client in all subsequent messages. Valid IDs are 1-16 (MAXCLIENTS).

### 6.4 Message: LoginDenied (ID 3)

**Direction:** Server → Client  
**Format:** Protobuf

```protobuf
message LoginDenied {
    optional string reason = 1;      // Human-readable reason
    optional Connection forward_to = 2; // Redirect to another server
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Server rejects connection (server full, banned, version incompatible, etc.).

### 6.5 Message: Logout (ID 7)

**Direction:** Client → Server OR Server → Client  
**Format:** Protobuf

```protobuf
message Logout {
    optional uint32 my_id = 200;  // Network ID of disconnecting party
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Clean disconnect notification.

---

## 7. Game Messages

### 7.1 Acknowledgment (ID 1)

**Direction:** Bidirectional  
**Format:** Protobuf

```protobuf
message Ack {
    repeated uint32 ack_ids = 1;  // Message IDs being acknowledged
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Acknowledge receipt of important messages. Used for reliable delivery over UDP.

**Reliable Messages:** Some messages (login, logout, critical game events) require acknowledgment. If not acknowledged within a timeout, they are retransmitted.

### 7.2 Version Control (ID 10)

**Direction:** Server → Client  
**Format:** Protobuf

```protobuf
message VersionControl {
    optional VersionSync version = 1;  // Active protocol version
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Server announces the common protocol version that all connected clients must use. Sent when a new client connects to ensure all clients use compatible versions.

### 7.3 Console Message (ID 8)

**Direction:** Server → Client  
**Format:** Protobuf

```protobuf
message ConsoleMessage {
    optional string message = 1;  // Message to display in console
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Server sends text message to client's console. Supports color codes and formatting.

### 7.4 Center Message (ID 9)

**Direction:** Server → Client  
**Format:** Protobuf

```protobuf
message CenterMessage {
    optional string message = 1;  // Message to display in center of screen
    optional bool legacy_message_end_marker = 20000;
}
```

**Purpose:** Server sends prominent center-screen message (match start, player death, etc.).

### 7.5 Game State Synchronization

Game state is synchronized using the NetObject system. Each game object (cycles, walls, players) has a network ID and sends periodic sync messages.

**Key Concepts:**
- **NetObjects:** Game entities with network IDs
- **Sync Messages:** Periodic state updates
- **Descriptors:** Each object type has a message descriptor (ID 200+)
- **Priority:** Critical updates sent more frequently

**NetObject Lifecycle:**
1. **Creation:** Server sends creation message with object ID
2. **Synchronization:** Periodic sync messages with state updates
3. **Destruction:** Server sends destruction message

---

## 8. Message Catalog

### 8.1 Complete Message ID List

| ID | Name | Direction | Format | Purpose |
|----|------|-----------|--------|---------|
| 0 | Default/Dummy | Any | Both | Fallback handler |
| 1 | Ack | Bidirectional | Protobuf | Acknowledge messages |
| 3 | LoginDenied | Server→Client | Protobuf | Reject connection |
| 4 | LoginIgnored | Server→Client | Protobuf | Ignore login (flood protection) |
| 5 | LoginAccepted | Server→Client | Protobuf | Accept connection |
| 6 | Login | Client→Server | Protobuf | Request connection |
| 7 | Logout | Bidirectional | Protobuf | Disconnect |
| 8 | ConsoleMessage | Server→Client | Protobuf | Console text |
| 9 | CenterMessage | Server→Client | Protobuf | Center screen text |
| 10 | VersionControl | Server→Client | Protobuf | Protocol version sync |
| 12 | VersionOverride | Bidirectional | Protobuf | Override version |
| 50 | SmallServerInfo | Server→Client | Protobuf | Minimal server info |
| 51 | BigServerInfo | Server→Client | Protobuf | Full server info |
| 52 | RequestSmallServerInfo | Client→Server | Protobuf | Request server info |
| 53 | RequestBigServerInfo | Client→Server | Protobuf | Request detailed info |
| 54 | BigServerInfoMaster | Master→Client | Protobuf | Server info from master |
| 55 | RequestBigServerInfoMaster | Client→Master | Protobuf | Request from master |
| 200+ | Game Objects | Bidirectional | Varies | Game state synchronization |

### 8.2 Message Priority

Messages are prioritized for bandwidth management:

- **Urgent (0):** Login, logout, critical state changes
- **High (1-10):** Important game events, player actions
- **Normal (11-100):** Regular updates, chat messages  
- **Low (101+):** Non-critical information

### 8.3 Reliable vs. Unreliable

- **Reliable:** Login, logout, version control (require ACK)
- **Unreliable:** Position updates, frequent game state (no ACK needed)

---

## 9. Implementation Guide

### 9.1 Minimal Server Discovery (Python)

```python
import socket
import struct

def discover_servers(timeout=2.0):
    """Discover Armagetron servers on LAN"""
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
    sock.settimeout(timeout)
    
    # Build RequestSmallServerInfo (protobuf)
    # Message ID: 0x8034 (52 with protobuf flag)
    message_id = struct.pack('>H', 0x8034)
    # Protobuf: transaction=0, marker=true
    payload = bytes([0x08, 0x00, 0xA0, 0x9C, 0x02, 0x01])
    
    # Broadcast request
    sock.sendto(message_id + payload, ('255.255.255.255', 4534))
    
    servers = []
    while True:
        try:
            data, addr = sock.recvfrom(1024)
            msg_id = struct.unpack('>H', data[0:2])[0]
            if (msg_id & 0x7FFF) == 50:  # SmallServerInfo
                servers.append(addr[0])
                print(f"Found server at {addr[0]}:{addr[1]}")
        except socket.timeout:
            break
    
    return servers

# Usage
servers = discover_servers()
print(f"Found {len(servers)} server(s)")
```

### 9.2 Minimal Server Implementation (Python)

```python
import socket
import struct

def run_discovery_server(port=4534, server_name="My Server"):
    """Respond to server discovery requests"""
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(('0.0.0.0', port))
    
    print(f"Server listening on port {port}...")
    
    while True:
        data, addr = sock.recvfrom(1024)
        msg_id = struct.unpack('>H', data[0:2])[0]
        msg_id &= 0x7FFF
        
        if msg_id == 52:  # RequestSmallServerInfo
            # Build SmallServerInfo response
            response_id = struct.pack('>H', 0x8032)  # ID 50 + protobuf
            # Protobuf payload with port, hostname, transaction
            payload = bytes([
                0x0A, 0x07,  # field 1 (base): length 7
                0x08, 0xB6, 0x23,  # port: 4534
                0xA0, 0x9C, 0x02, 0x01,  # marker
                0x10, 0x00,  # transaction: 0
                0xA0, 0x9C, 0x02, 0x01   # marker
            ])
            sock.sendto(response_id + payload, addr)
            print(f"Responded to discovery from {addr}")

# Usage
run_discovery_server()
```

### 9.3 Using Protocol Buffers (Python)

```python
# First, compile the .proto files:
# protoc --python_out=. nServerInfo.proto nNetwork.proto

import nServerInfo_pb2
import nNetwork_pb2

def build_big_server_info(name, players, max_players):
    """Build a BigServerInfo message using protobuf"""
    msg = nServerInfo_pb2.BigServerInfo()
    
    # Base info
    msg.base.port = 4534
    msg.base.hostname = ""
    msg.base.legacy_message_end_marker = True
    
    # Server details
    msg.name = name
    msg.users = players
    msg.max_users = max_players
    msg.release = "0.4.0"
    msg.version.min = 25
    msg.version.max = 30
    msg.version.legacy_message_end_marker = True
    msg.legacy_message_end_marker = True
    
    return msg.SerializeToString()

# Usage
payload = build_big_server_info("Test Server", 2, 16)
# Send with message ID 0x8033 (51 + protobuf flag)
```

### 9.4 Complete Server Discovery Example (Go)

```go
package main

import (
    "fmt"
    "net"
    "encoding/binary"
)

func handleServerInfo(conn *net.UDPConn, addr *net.UDPAddr) {
    // Build SmallServerInfo protobuf response
    response := []byte{
        0x80, 0x32, // Message ID (50 + protobuf flag)
        0x0A, 0x07, // field 1: embedded message length 7
        0x08, 0xB6, 0x23, // port: 4534
        0xA0, 0x9C, 0x02, 0x01, // marker
        0x10, 0x00, // transaction: 0
        0xA0, 0x9C, 0x02, 0x01, // marker
    }
    conn.WriteToUDP(response, addr)
}

func main() {
    addr, _ := net.ResolveUDPAddr("udp", ":4534")
    conn, _ := net.ListenUDP("udp", addr)
    defer conn.Close()
    
    buffer := make([]byte, 1024)
    for {
        n, addr, _ := conn.ReadFromUDP(buffer)
        if n < 2 {
            continue
        }
        
        msgID := binary.BigEndian.Uint16(buffer[0:2])
        msgID &= 0x7FFF
        
        if msgID == 52 { // RequestSmallServerInfo
            handleServerInfo(conn, addr)
            fmt.Printf("Responded to %s\n", addr)
        }
    }
}
```

---

## 10. Advanced Topics

### 10.1 Master Server Protocol

Master servers act as central registries for global server lists.

**Registration Flow:**
1. Server sends SmallServerInfo to master periodically (every 5 minutes)
2. Master stores server information
3. Clients query master with RequestBigServerInfoMaster
4. Master responds with cached BigServerInfo for requested server

**Master Server Addresses:**
- `master1.armagetronad.org:4534`
- `master2.armagetronad.org:4534`
- Additional mirrors as configured

### 10.2 Authentication System

Armagetron supports multi-authority authentication:

```protobuf
message Hash {
    optional string method = 1;  // Hash method ("md5", "sha256")
    optional bytes hash = 2;     // Hash value
}

message Authority {
    optional string name = 1;         // Authority name
    optional string authority_key = 2; // Public key
}
```

**Authentication Flow:**
1. Client provides authentication methods in Login message
2. Server challenges client with random token
3. Client signs token with private key
4. Server verifies signature using authority's public key

### 10.3 Bandwidth Management

The protocol includes sophisticated bandwidth control:

- **Prioritization:** Critical messages sent first
- **Rate Limiting:** Configurable per-client bandwidth limits
- **Diff Compression:** Send only changed fields
- **Adaptive Update Rate:** Reduce updates when bandwidth constrained

### 10.4 NetObject System

Game objects use descriptor-based synchronization:

```
Creation Message (once):
  - Object type descriptor ID
  - Object network ID
  - Initial state

Sync Messages (periodic):
  - Object network ID
  - Changed fields only (diff)
  - Priority-based frequency

Destruction Message (once):
  - Object network ID
```

**Common Object Types:**
- **Player NetID:** Player information (ID 200-300 range)
- **Cycle:** Light cycle position/direction (ID 300-400 range)
- **Wall:** Wall segments (ID 400-500 range)
- **Zone:** Game zones (ID 500-600 range)

---

## 11. Troubleshooting

### 11.1 Server Not Visible in Browser

**Symptoms:** Client can't find server on LAN.

**Checklist:**
1. ✓ Server listening on port 4534 (UDP)
2. ✓ Firewall allows UDP broadcast
3. ✓ Using protobuf format for modern clients (0x8000 flag set)
4. ✓ Correct big-endian message ID in header
5. ✓ Valid protobuf payload with legacy_message_end_marker

**Debug:**
```bash
# Capture packets to verify format
tcpdump -i any -X udp port 4534

# Send manual discovery request
echo -ne '\x80\x34\x08\x00\xA0\x9C\x02\x01' | nc -u -w1 -b 255.255.255.255 4534
```

### 11.2 Login Fails

**Symptoms:** LoginDenied or no response to Login message.

**Common Causes:**
- **Version Mismatch:** Client version range doesn't overlap with server
- **Server Full:** max_users reached
- **IP Ban:** Client IP is banned
- **Invalid Protobuf:** Malformed Login message

**Version Check:**
```
Client: min=20, max=30
Server: min=25, max=35
Result: Compatible (overlap: 25-30)

Client: min=10, max=20
Server: min=25, max=35
Result: Incompatible (no overlap)
```

### 11.3 Protobuf Parsing Errors

**Symptoms:** Messages cause parse errors or are ignored.

**Solutions:**
1. Verify protobuf compiler version matches .proto files
2. Check field 20000 (legacy_message_end_marker) is present
3. Ensure varint encoding is correct
4. Validate embedded message lengths

**Varint Validation:**
```python
def decode_varint(data, offset):
    """Decode protobuf varint"""
    result = 0
    shift = 0
    while True:
        byte = data[offset]
        result |= (byte & 0x7F) << shift
        offset += 1
        if (byte & 0x80) == 0:
            break
        shift += 7
    return result, offset
```

---

## 12. References

- **Source Code:** https://gitlab.com/armagetronad/armagetronad
- **Protocol Buffers:** https://developers.google.com/protocol-buffers
- **Wiki:** http://wiki.armagetronad.org/
- **Forums:** https://forums.armagetronad.org/

---

## Appendix A: Protocol Buffer Field IDs

### SmallServerInfo Family
- **SmallServerInfoBase (embedded):**
  - 1: port (int32)
  - 2: hostname (string)
  - 20000: legacy_message_end_marker (bool)

- **SmallServerInfo:**
  - 1: base (SmallServerInfoBase)
  - 2: transaction (int32)
  - 20000: legacy_message_end_marker (bool)

- **RequestSmallServerInfo:**
  - 1: transaction (int32)
  - 20000: legacy_message_end_marker (bool)

### BigServerInfo
- 1: base (SmallServerInfoBase)
- 2: name (string)
- 3: users (int32)
- 4: version (VersionSync)
- 5: release (string)
- 6: max_users (int32)
- 7: usernames (string)
- 8: options (string)
- 9: url (string)
- 10: global_ids (string)
- 11: settings (SettingsDigest)
- 20000: legacy_message_end_marker (bool)

### SettingsDigest
- 1: flags (uint32)
- 2: min_play_time_total (int32)
- 3: min_play_time_online (int32)
- 4: min_play_time_team (int32)
- 5: cycle_delay (float)
- 6: acceleration (float)
- 7: rubber_wall_hump (float)
- 8: rubber_hit_wall_ratio (float)
- 9: walls_length (float)
- 20000: legacy_message_end_marker (bool)

### Login/Connection Messages
- **Login:**
  - 1: rate (uint32)
  - 2: big_brother (string)
  - 3: version (VersionSync)
  - 4: authentication_methods (string)
  - 5: token (Hash)
  - 6: options (EncodingOptions)
  - 20000: legacy_message_end_marker (bool)

- **LoginAccepted:**
  - 1: net_id (uint32)
  - 2: version (VersionSync)
  - 3: address (string)
  - 4: token (Hash)
  - 5: options (EncodingOptions)
  - 20000: legacy_message_end_marker (bool)

- **VersionSync:**
  - 1: min (int32)
  - 2: max (int32)
  - 20000: legacy_message_end_marker (bool)

---

## Appendix B: Legacy Stream Format (Not Recommended)

For historical reference only. **Do not use for new implementations.**

The legacy stream format uses nMessage serialization with unsigned shorts as the base unit. All multi-byte values are in the platform's native byte order (typically little-endian).

**String Encoding:**
```
[length_including_null(uint16)] [char_pairs(uint16)...] [last_char_if_odd(uint16)]
```

**Integer Encoding:**
```
[low_16_bits(uint16)] [high_16_bits(int16)]
```

**Float Encoding (REAL):**
Custom 32-bit format with 25-bit mantissa, 1-bit sign, 6-bit exponent, stored as two uint16 values.

---

## Appendix C: Version History

- **0.2.8 and earlier:** Legacy stream format only, basic UDP protocol
- **0.2.9.0:** Protocol Buffers introduced, backward compatible, improved discovery
- **0.3.0:** Enhanced authentication, improved NetObject system
- **0.4.0:** Protobuf preferred, master server improvements, better bandwidth management
- **Future:** Legacy format may be deprecated in favor of protobuf-only

---

**End of Protocol Specification**

Last Updated: October 2025




---


# Legacy Notes

Legacy Streaming Protocol Packet Structure

For legacy stream messages, the packet format is:

[Descriptor ID (2 bytes)] [Message ID (2 bytes)] [Data Length (2 bytes)] [Data (N × 2 bytes)]

Breakdown:
1. Descriptor ID (2 bytes, big-endian): Message type identifier (0-399)
2. Message ID (2 bytes, big-endian): Unique ID for tracking/acknowledgment
3. Data Length (2 bytes, big-endian): Number of shorts (16-bit values) in the data
4. Data (variable): Array of 16-bit values

Reading/writing is done in nStreamMessage.cpp:558-587:
- Uses nBinaryWriter::WriteShort() which writes big-endian (high byte first, then low byte)
- Data consists of encoded values (shorts, ints, floats, strings) packed as 16-bit values

  ---
Protocol Buffer (Modern) Packet Structure

For Protocol Buffer messages (like SmallServerInfo), the format is:

[Descriptor ID (2 bytes)] [Header (variable)] [Protobuf Payload (variable)]

Breakdown:

1. Descriptor ID (2 bytes, big-endian)

- Has bit 0x8000 set to indicate it's a protobuf message
- For SmallServerInfo: 50 | 0x8000 = 0x8032

2. Header (variable length, from nProtoBuf.cpp:119-150)

[Flags (1 byte)] [MessageID (0 or 2 bytes)] [CacheRef (variable)] [Length (variable)]

- Flags (1 byte):
  - Bit 0 (0x01): FLAG_MessageID - messageID field present
  - Bit 1 (0x02): FLAG_CacheRef - cacheRef field present
- MessageID (2 bytes, big-endian): Only present if FLAG_MessageID is set
- CacheRef (variable): Only present if FLAG_CacheRef is set
  - If messageID present: Variable-length uint (relative reference)
  - If no messageID: 2 bytes (absolute reference)
- Length (variable): Variable-length uint encoding the protobuf payload size

3. Protobuf Payload

- Serialized Protocol Buffer message using standard protobuf encoding

  ---
Creating a SmallServerInfo Packet (Message ID 50)

Message Structure

From nServerInfo.proto:28-36:
message SmallServerInfo {
optional SmallServerInfoBase base = 1;
optional int32 transaction = 2;
}

message SmallServerInfoBase {
optional int32  port = 1;
optional string hostname = 2;
}

Code Example

From nServerInfo.cpp:1267-1281:

// Create the message
tJUST_CONTROLLED_PTR< nProtoBufMessage< Network::SmallServerInfo > >
ret = sn_smallServerInfoDescriptor.CreateMessage();

// Fill in server info
nServerInfoBase info;
info.GetFrom( sn_Connections[receiverID].socket );

// Write to protobuf
info.WriteSync( *ret->AccessProtoBuf().mutable_base() );

// Set transaction number
ret->AccessProtoBuf().set_transaction(0);

// Send it
ret->SendImmediately(receiverID, false);

WriteSync Implementation

From nServerInfo.cpp:2787-2791:
void nServerInfoBase::WriteSync( Network::SmallServerInfoBase & info ) const
{
info.set_port( port_ );           // Write the port
info.set_hostname( connectionName_ );  // Write the hostname
}

Binary Packet Example

A typical SmallServerInfo packet might look like:
0x80 0x32          // Descriptor ID (50 | 0x8000)
0x03               // Flags (messageID + length present)
0x12 0x34          // Message ID (example: 0x1234)
0x15               // Length (21 bytes protobuf payload)
[21 bytes of protobuf data containing port, hostname, transaction]

Protobuf Encoding

The protobuf payload uses standard Protocol Buffer encoding:
- Field 1 (base): Nested SmallServerInfoBase
  - Field 1 (port): varint encoded int32
  - Field 2 (hostname): length-delimited string
- Field 2 (transaction): varint encoded int32

Example protobuf bytes:
0x0A 0x0E           // Field 1 (base), wire type 2 (length-delimited), length 14
0x08 0xB5 0x46    // Field 1 (port), varint 4533
0x12 0x09         // Field 2 (hostname), length 9
[9 bytes: "localhost"]
0x10 0x00           // Field 2 (transaction), value 0

  ---
Key Implementation Files

- Packet reading: src/network/nNetwork.cpp:672-746
- Stream message format: src/network/nStreamMessage.cpp
- Protobuf format: src/network/nProtoBuf.cpp:323-402
- Binary I/O: src/network/nBinary.h
- SmallServerInfo definition: src/protobuf/nServerInfo.proto
- SmallServerInfo handler: src/network/nServerInfo.cpp:1107-1283




---

Notes on packet 7 from client

Message Structure

  Incoming from client (descriptor 7):
  - Descriptor: 7 (2 bytes, big endian)
  - Message ID: varies (2 bytes, big endian)
  - Data Length: 1 (2 bytes, big endian) - means 1 short = 2 bytes
  - Data: Client's network ID (2 bytes, big endian, unsigned short)

  The packet structure would look like:
  [0x00, 0x07]  // descriptor 7 = logout
  [msg_id]      // message ID from client
  [0x00, 0x01]  // data length = 1 short
  [client_id]   // the client's network ID (unsigned short)
  [sender_id]   // sender ID field at end

  What the Server Does

  Looking at lines 2186-2208, when the server receives logout:
  1. Logs the logout message (line 2199: "$network_logout_server")
  2. Acknowledges all pending messages from that client (line 2202)
  3. Disconnects the user (line 2206: calls sn_DisconnectUser)

  Response

  No explicit response is sent back. The server just:
  - Cleans up the connection internally
  - The client detects the disconnection when the socket closes or times out

  In Your Go Server

  You would decode it like:

  // After reading descriptor 7
  var clientNetID uint16
  // Read 2 bytes and convert from big endian
  clientNetID = binary.BigEndian.Uint16(data[0:2])

  // Log it
  s.Config.Logger.Printf("Client %d logging out\n", clientNetID)

  // Clean up any connection state for this client
  // (close sockets, remove from active clients list, etc.)

  You don't need to send a response - just clean up server state.
