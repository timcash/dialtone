# 01 Embedded NATS + topic publish

### Conditions
Embedded broker starts and wildcard listener captures topic logs.

### Results
```text
Embedded NATS started at nats://127.0.0.1:44522 and NATS messages verified.
```

# 02 Listener filtering (error.topic)

### Conditions
Listener on logs.error.topic only receives error topic messages.

### Results
```text
Verified error-topic filtering via NATS: logs.error.topic received the message.
```

# 04 Two-process pingpong via dialtone logs

### Conditions
Two ./dialtone.sh logs pingpong processes exchange at least 3 ping/pong rounds on one topic.

### Results
```text
Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic (verified via NATS).
```

# 05 Example plugin binary imports logs library

### Conditions
A built binary under logs/src_v1/test imports logs library, auto-starts embedded NATS when missing, and publishes topic messages.

### Results
```text
Verified example plugin binary imports logs library, and verified via both NATS messages and file listener.
```

# 03 Finalize artifacts

### Conditions
Artifacts exist and include captured topic lines.

### Results
```text
Suite finalized. Verification transitioned to NATS topics.
```

