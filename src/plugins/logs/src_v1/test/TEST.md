# 01 Embedded NATS + topic publish

### Conditions
Embedded broker starts and wildcard listener captures topic logs.

### Results
```text
Embedded NATS started at nats://127.0.0.1:45128 and wildcard listener captured logs.info.topic + logs.error.topic.
```

# 02 Listener filtering (error.topic)

### Conditions
Listener on logs.error.topic only receives error topic messages.

### Results
```text
Verified error-topic listener filtering: error.log only contains logs.error.topic records.
```

# 04 Two-process pingpong via dialtone logs

### Conditions
Two ./dialtone.sh logs pingpong processes exchange at least 3 ping/pong rounds on one topic.

### Results
```text
Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.
```

# 05 Example plugin binary imports logs library

### Conditions
A built binary under logs/src_v1/test imports logs library, auto-starts embedded NATS when missing, and publishes topic messages.

### Results
```text
Verified example plugin binary imports logs library, auto-starts embedded NATS when missing, and publishes/listens on topic.
```

# 03 Finalize artifacts

### Conditions
Artifacts exist and include captured topic lines.

### Results
```text
Artifacts ready: test.log=4 lines, error.log=1 lines.
```

