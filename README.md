# MQTT X - High-Performance Multi-Session MQTT Client Library

[![PkgGoDev](https://pkg.go.dev/badge/github.com/darkit/mqttx.svg)](https://pkg.go.dev/github.com/darkit/mqttx)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkit/mqttx)](https://goreportcard.com/report/github.com/darkit/mqttx)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/darkit/mqttx/blob/master/LICENSE)

## 🚀 Introduction

MQTT X is a high-performance multi-session MQTT client library designed for Go applications. With deep optimizations, it provides exceptional performance, clean APIs, and powerful features.

## ✨ Key Features

### 🏗️ Architecture Optimizations
- **Builder Pattern**: Fluent API design that simplifies configuration
- **Object Pool Technology**: Automatic memory management with 28.5x performance improvement and zero allocations
- **Atomic Operations**: 4M+ atomic operations/sec ensuring concurrent safety
- **Unified Error Handling**: Structured error types with enhanced error information quality

### 🎯 Functional Features
- **Multi-Session Management**: Concurrent handling of multiple MQTT connections
- **Message Forwarding System**: Cross-session and cross-topic message forwarding
- **Auto-Reconnection**: Built-in exponential backoff reconnection strategy
- **TLS/SSL Support**: Certificate-based secure communication
- **Session Persistence**: Support for memory, file, and Redis storage
- **Real-time Monitoring**: Detailed performance and health metrics

### 🔧 Technical Features
- **Thread-safe Design**: All operations are concurrency-safe
- **Performance Monitoring**: Built-in metrics collection and performance analysis
- **Flexible Configuration**: Rich configuration options and tuning parameters
- **Error Recovery**: Intelligent error detection and recovery mechanisms

## Installation

```bash
go get github.com/darkit/mqttx
```

## 🚀 Quick Start

### Basic Usage

```go
package main

import (
    "log"
    "time"
    "github.com/darkit/mqttx"
)

func main() {
    // Create session manager
    manager := mqttx.NewSessionManager()
    defer manager.Close()

    // Use Builder pattern to create session
    opts, err := mqttx.QuickConnect("prod-device", "broker.example.com:1883").
        Auth("username", "password").
        KeepAlive(60).
        AutoReconnect().
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // Add session and connect
    if err := manager.AddSession(opts); err != nil {
        log.Fatal(err)
    }

    if err := manager.ConnectAll(); err != nil {
        log.Fatal(err)
    }

    // Wait for connection to complete
    if err := manager.WaitForAllSessions(30 * time.Second); err != nil {
        log.Printf("Connection warnings: %v", err)
    }

    // Publish and subscribe messages
    session, _ := manager.GetSession("prod-device")
    
    // Subscribe to topic
    handler := func(topic string, payload []byte) {
        log.Printf("Received: %s = %s", topic, string(payload))
    }
    session.Subscribe("sensors/+/temperature", 1, handler)
    
    // Publish message
    session.Publish("sensors/room1/temperature", []byte("23.5"), 1, false)
    
    select {} // Keep running
}
```

## 📚 Core Concepts

### Builder Pattern

MQTT X provides fluent APIs to simplify configuration:

```go
// Quick connect
opts, err := mqttx.QuickConnect("session-name", "localhost:1883").Build()

// Secure connect
opts, err := mqttx.SecureConnect("secure-session", "ssl://broker:8883", "/path/to/ca.crt").
    Auth("user", "pass").
    KeepAlive(60).
    Build()

// Complex configuration
opts, err := mqttx.NewSessionBuilder("production-session").
    Brokers("tcp://broker1:1883", "tcp://broker2:1883").
    ClientID("client-001").
    Auth("admin", "secret").
    TLS("/etc/ssl/ca.crt", "/etc/ssl/client.crt", "/etc/ssl/client.key", false).
    Performance(16, 5000).
    RedisStorage("localhost:6379").
    Subscribe("sensors/+", 1, handler).
    Build()
```

### Message Forwarding

Automatic message forwarding between sessions:

```go
// Create forwarder
config, err := mqttx.NewForwarderBuilder("sensor-forwarder").
    Source("sensor-session", "sensors/+/temperature").
    Target("storage-session").
    QoS(1).
    MapTopic("sensors/room1/temperature", "storage/room1/temp").
    Build()

forwarder, err := mqttx.NewForwarder(config, manager)
forwarder.Start()
```

### Error Handling

Unified error handling mechanism:

```go
// Check error types
if mqttx.IsTemporary(err) {
    // Temporary error, can retry
    log.Printf("Temporary error: %v", err)
} else if mqttx.IsTimeout(err) {
    // Timeout error
    log.Printf("Timeout error: %v", err)
}

// Create custom error
err := mqttx.NewConnectionError("connection failed", originalErr).
    WithSession("my-session").
    WithContext("retry_count", 3)
```

## 📊 Performance Metrics

MQTT X performance on standard hardware:

- **Message Throughput**: 100K+ messages/sec
- **Metric Operations**: 4M+ atomic operations/sec
- **Object Pool Optimization**: 28.5x performance improvement
- **Memory Efficiency**: < 5 bytes per metric object
- **Forwarder Performance**: 500K+ lifecycles/sec

### Performance Monitoring

```go
// Global metrics
globalMetrics := manager.GetMetrics()
log.Printf("Total messages: %d, Errors: %d", 
    globalMetrics.TotalMessages, globalMetrics.ErrorCount)

// Session metrics
sessionMetrics := session.GetMetrics()
log.Printf("Sent: %d, Received: %d", 
    sessionMetrics.MessagesSent, sessionMetrics.MessagesReceived)

// Forwarder metrics
forwarderMetrics := forwarder.GetMetrics()
log.Printf("Forwarded: %d, Dropped: %d", 
    forwarderMetrics.MessagesSent, forwarderMetrics.MessagesDropped)
```

## Core Components

### Session Manager

The session manager (`Manager`) is the central component that handles multiple MQTT sessions:

```go
// Create a new manager
m := manager.NewSessionManager()

// Add a session
err := m.AddSession(&manager.Options{...})

// Get session status
status := m.GetAllSessionsStatus()

// Remove a session
err := m.RemoveSession("session-name")

// List all sessions
sessions := m.ListSessions()
```

### Connection Management

The manager provides connection waiting mechanisms to ensure sessions are ready before operations:

```go
// Wait for a specific session to connect
err := m.AddSession(opts)
if err != nil {
    log.Fatal(err)
}

// Wait up to 30 seconds for session to be ready
if err := m.WaitForSession("prod-device", 30*time.Second); err != nil {
    log.Fatal(err)
}

// Or wait for all sessions to be ready
if err := m.WaitForAllSessions(30*time.Second); err != nil {
    log.Fatal(err)
}
```

### Message Handling

Four flexible message handling patterns are available:

1. **Handle** - Global callback-based handling:
```go
route := m.Handle("topic/#", func(msg *manager.Message) {
    log.Printf("Received: %s", msg.PayloadString())
})
defer route.Stop()
```

2. **HandleTo** - Session-specific callback handling:
```go
route, err := m.HandleTo("session-name", "topic/#", func(msg *manager.Message) {
    log.Printf("Received on session: %s", msg.PayloadString())
})
defer route.Stop()
```

3. **Listen** - Channel-based message reception:
```go
messages, route := m.Listen("topic/#")
go func() {
    for msg := range messages {
        log.Printf("Received: %s", msg.PayloadString())
    }
}()
defer route.Stop()
```

4. **ListenTo** - Session-specific channel reception:
```go
messages, route, err := m.ListenTo("session-name", "topic/#")
go func() {
    for msg := range messages {
        log.Printf("Received: %s", msg.PayloadString())
    }
}()
defer route.Stop()
```

### Message Forwarder

The message forwarder allows automatic message forwarding between different sessions and topics, with support for filtering, transformation, and metadata injection:

```go
// Create forwarder manager
forwarderManager := mqttx.NewForwarderManager(manager)

// Configure forwarder
forwarderConfig := mqttx.ForwarderConfig{
    Name:           "temperature-forwarder",
    SourceSessions: []string{"source-session1", "source-session2"},
    SourceTopics:   []string{"sensors/+/temperature"},
    TargetSession:  "target-session",
    TopicMapping:   map[string]string{
        "sensors/living-room/temperature": "processed/temperature/living-room",
    },
    QoS:            1,
    Metadata: map[string]interface{}{
        "forwarded_by": "temperature-forwarder",
        "timestamp":    time.Now().Unix(),
    },
    Enabled:        true,
}

// Add and start forwarder
forwarder, err := forwarderManager.AddForwarder(forwarderConfig)
if err != nil {
    log.Fatal(err)
}

// Get forwarder metrics
metrics := forwarder.GetMetrics()
log.Printf("Messages forwarded: %d", metrics["messages_forwarded"])

// Stop all forwarders
forwarderManager.StopAll()
```

The forwarder supports the following features:

1. **Multi-source Forwarding** - Subscribe to messages from multiple sessions
2. **Topic Mapping** - Map source topics to different target topics
3. **Message Filtering** - Filter messages based on topic or content
4. **Message Transformation** - Transform message content before forwarding
5. **Metadata Injection** - Add metadata to forwarded messages
6. **Performance Metrics** - Provide detailed forwarding statistics

### Event System

Monitor session lifecycle and state changes with detailed event information:

```go
// Monitor connection status
m.OnEvent("session_ready", func(event manager.Event) {
    log.Printf("Session %s is ready for operations", event.Session)
})

// Monitor state changes
m.OnEvent("session_state_changed", func(event manager.Event) {
    stateData := event.Data.(map[string]interface{})
    log.Printf("Session %s state changed from %v to %v",
        event.Session,
        stateData["old_state"],
        stateData["new_state"])
})
```

Available Events:
- `session_connecting` - Session is attempting to connect
- `session_connected` - Session has successfully connected
- `session_ready` - Session is ready for operations
- `session_disconnected` - Session has disconnected (includes error info if any)
- `session_reconnecting` - Session is attempting to reconnect
- `session_added` - New session has been added to the manager
- `session_removed` - Session has been removed from the manager
- `session_state_changed` - Session state has changed

Event Data Structure:
```go
type Event struct {
    Type      string      // Event type
    Session   string      // Session name
    Data      interface{} // Additional event data
    Timestamp time.Time   // Event timestamp
}
```

Common Event Data Contents:
- `session_connected`: Connection details
- `session_disconnected`: Error information (if any)
- `session_state_changed`: Map containing "old_state" and "new_state"
- `session_reconnecting`: Reconnection attempt count
- `session_ready`: Session configuration summary

### Advanced Configuration

#### TLS Security

```go
opts := &manager.Options{
    Name:     "secure-session",
    Brokers:  []string{"ssl://broker.example.com:8883"},
    ClientID: "secure-client-001",
    TLS: &manager.TLSConfig{
        CAFile:     "/path/to/ca.crt",
        CertFile:   "/path/to/client.crt",
        KeyFile:    "/path/to/client.key",
        SkipVerify: false,
    },
}
```

#### Performance Tuning

```go
opts := &manager.Options{
    Performance: &manager.PerformanceOptions{
        WriteBufferSize:    4096,
        ReadBufferSize:     4096,
        MessageChanSize:    1000,
        MaxMessageSize:     32 * 1024,
        MaxPendingMessages: 5000,
        WriteTimeout:       time.Second * 30,
        ReadTimeout:        time.Second * 30,
    },
}
```

#### Session Persistence

```go
opts := &manager.Options{
    ConnectProps: &manager.ConnectProps{
        PersistentSession: true,
        ResumeSubs:       true,
    },
}
```

#### Metrics Collection

Monitor session and manager performance:

```go
// Get manager-level metrics
metrics := m.GetMetrics()

// Get session-specific metrics
session, _ := m.GetSession("session-name")
sessionMetrics := session.GetMetrics()

// Get all forwarder metrics
forwarderMetrics := forwarderManager.GetAllMetrics()
```

##### Prometheus Integration

Expose metrics in Prometheus format via HTTP endpoint:

```go
// Create HTTP server to expose Prometheus metrics
go func() {
    promExporter := manager.NewPrometheusExporter("mqtt")
    
    http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        var output strings.Builder
        
        // Collect manager metrics
        metrics := m.GetMetrics()
        output.WriteString(promExporter.Export(metrics))
        
        // Collect all session metrics
        for _, name := range m.ListSessions() {
            if session, err := m.GetSession(name); err == nil {
                output.WriteString(session.PrometheusMetrics())
            }
        }
        
        w.Header().Set("Content-Type", "text/plain")
        fmt.Fprint(w, output.String())
    })
    
    log.Printf("Starting metrics server on :2112")
    http.ListenAndServe(":2112", nil)
}()
```

Add scrape target in Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'mqtt_metrics'
    static_configs:
      - targets: ['localhost:2112']
    scrape_interval: 15s
```

Available Prometheus metrics include:

Message Metrics:
- `mqtt_session_messages_sent_total` - Total messages sent
- `mqtt_session_messages_received_total` - Total messages received
- `mqtt_session_bytes_sent_total` - Total bytes sent
- `mqtt_session_bytes_received_total` - Total bytes received
- `mqtt_session_message_rate` - Current messages per second
- `mqtt_session_avg_message_rate` - Average messages per second since start
- `mqtt_session_bytes_rate` - Bytes per second

Status Metrics:
- `mqtt_session_connected` - Session connection status (0/1)
- `mqtt_session_status` - Session status code
- `mqtt_session_subscriptions` - Active subscription count
- `mqtt_session_errors_total` - Total error count
- `mqtt_session_reconnects_total` - Reconnection attempts

Timestamp Metrics:
- `mqtt_session_last_message_timestamp_seconds` - Unix timestamp of last message
- `mqtt_session_last_error_timestamp_seconds` - Unix timestamp of last error

Session Properties:
- `mqtt_session_persistent` - Persistent session flag (0/1)
- `mqtt_session_clean_session` - Clean session flag (0/1)
- `mqtt_session_auto_reconnect` - Auto reconnect flag (0/1)

All metrics include a `session="session-name"` label for filtering and aggregation by session.

## 🔧 Advanced Features

### Session Persistence

Support for multiple storage backends:

```go
// Memory storage (default, fastest)
opts := mqttx.NewSessionBuilder("memory-session").
    Broker("localhost:1883").
    Build()

// File storage
opts := mqttx.NewSessionBuilder("file-session").
    Broker("localhost:1883").
    FileStorage("/var/lib/mqttx").
    Build()

// Redis storage
opts := mqttx.NewSessionBuilder("redis-session").
    Broker("localhost:1883").
    RedisStorage("localhost:6379").
    RedisAuth("user", "pass", 1).
    Build()
```

### Performance Tuning

```go
// High-performance configuration
opts := mqttx.NewSessionBuilder("high-perf").
    Broker("localhost:1883").
    Performance(32, 10000).      // 32KB buffer, 10K pending messages
    MessageChannelSize(2000).    // 2K message channel
    KeepAlive(300).             // 5-minute keepalive
    Timeouts(10, 5).            // 10s connect, 5s write timeout
    Build()
```

### TLS Security

```go
// Secure connection
opts := mqttx.SecureConnect("secure-session", "ssl://broker:8883", "/path/to/ca.crt").
    Auth("username", "password").
    TLS("/path/to/ca.crt", "/path/to/client.crt", "/path/to/client.key", false).
    Build()
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run benchmarks
go test -bench=. -benchmem

# Run race condition tests
go test -race ./...

# Performance tests
go test -run TestPerformanceImprovement -v
```

## Best Practices

1. **Resource Management**
    - Always use `defer route.Stop()` for subscription cleanup
    - Implement proper error handling
    - Use meaningful session names and client IDs

2. **Performance Optimization**
    - Configure appropriate buffer sizes for your use case
    - Use session-specific subscriptions (`HandleTo`/`ListenTo`) when possible
    - Monitor metrics to identify bottlenecks
    - Compare current and average message rates to identify traffic patterns
    - Use metrics data for capacity planning and performance tuning

3. **Reliability**
    - Enable automatic reconnection for production use
    - Implement proper error handling and retry mechanisms
    - Use QoS levels appropriate for your use case

4. **Security**
    - Enable TLS in production environments
    - Use strong client authentication
    - Regularly rotate credentials

5. **Forwarder Usage**
    - Set appropriate buffer sizes for forwarders to avoid message loss
    - Use filters to reduce unnecessary message forwarding
    - Monitor forwarder metrics to detect issues early
    - Design appropriate topic mapping strategies for complex scenarios

## 📖 Documentation

- [GoDoc](https://pkg.go.dev/github.com/darkit/mqttx) - Complete API reference

## 🤝 Contributing

We welcome Issues and Pull Requests! Please ensure:

1. Code passes all tests
2. Follow existing code style
3. Add necessary test cases
4. Update relevant documentation

## 📄 License

This project is licensed under the [MIT License](LICENSE).

## 🏆 Acknowledgments

Thanks to the following projects for inspiration and support:

- [Eclipse Paho MQTT Go Client](https://github.com/eclipse/paho.mqtt.golang)
- [Go Redis](https://github.com/redis/go-redis)

---

**MQTT X** - Making MQTT client development simpler and more efficient!