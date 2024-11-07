# MQTT Session Manager

[![PkgGoDev](https://pkg.go.dev/badge/github.com/darkit/mqttx.svg)](https://pkg.go.dev/github.com/darkit/mqttx)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkit/mqttx)](https://goreportcard.com/report/github.com/darkit/mqttx)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/darkit/mqttx/blob/master/LICENSE)

## Introduction

A robust multi-session MQTT manager for Go applications that provides concurrent management of multiple MQTT connections. Built with reliability, flexibility, and performance in mind.

## Key Features

- 🔄 Multi-session Management: Concurrent handling of multiple MQTT connections
- 🔌 Automatic Reconnection: Built-in reconnection mechanism with configurable backoff
- 🔒 TLS/SSL Support: Secure communication with certificate-based authentication
- 📨 Flexible Message Routing: Multiple message handling patterns (sync/async)
- 📊 Metrics Collection: Detailed performance and health metrics
- 💾 Session Persistence: Optional session state persistence
- 🎯 Event System: Comprehensive event notification system
- 🛡️ Thread-safe Design: Concurrent operations safety

## Installation

```bash
go get github.com/darkit/mqttx
```

## Quick Start

```go
func main() {
    // Create session manager
    m := manager.NewSessionManager()

    // Configure session options
    opts := &manager.Options{
        Name:     "prod-device",
        Brokers:  []string{"tcp://broker.example.com:1883"},
        ClientID: "device-001",
        ConnectProps: &manager.ConnectProps{
            KeepAlive:     60,
            CleanSession:  true,
            AutoReconnect: true,
        },
    }

    // Add session and wait for readiness
    if err := m.AddSession(opts); err != nil {
        log.Fatal(err)
    }

    // Wait for session to be ready
    if err := m.WaitForSession("prod-device", 30*time.Second); err != nil {
        log.Fatal(err)
    }

    // Now safe to subscribe and publish
    route := m.Handle("sensors/+/temperature", func(msg *manager.Message) {
        log.Printf("Temperature reading: %s", msg.PayloadString())
    })
    defer route.Stop()

    err := m.PublishTo("prod-device", "sensors/room1/temperature", []byte("23.5"), 1)
    if err != nil {
        log.Printf("Publish failed: %v", err)
    }

    select {} // Keep running
}
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
- `mqtt_session_message_rate` - Messages per second
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

Resource Metrics:
- `mqtt_session_goroutines` - Number of goroutines
- `mqtt_session_heap_alloc_bytes` - Allocated heap memory in bytes
- `mqtt_session_heap_inuse_bytes` - In-use heap memory in bytes

All metrics include a `session="session-name"` label for filtering and aggregation by session.

## Best Practices

1. **Resource Management**
    - Always use `defer route.Stop()` for subscription cleanup
    - Implement proper error handling
    - Use meaningful session names and client IDs

2. **Performance Optimization**
    - Configure appropriate buffer sizes for your use case
    - Use session-specific subscriptions (`HandleTo`/`ListenTo`) when possible
    - Monitor metrics to identify bottlenecks

3. **Reliability**
    - Enable automatic reconnection for production use
    - Implement proper error handling and retry mechanisms
    - Use QoS levels appropriate for your use case

4. **Security**
    - Enable TLS in production environments
    - Use strong client authentication
    - Regularly rotate credentials

## License

MIT License - see the [LICENSE](LICENSE) file for details