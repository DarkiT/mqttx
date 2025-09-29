# MQTTX - 高性能多会话 MQTT 客户端库

[![PkgGoDev](https://pkg.go.dev/badge/github.com/darkit/mqttx.svg)](https://pkg.go.dev/github.com/darkit/mqttx)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkit/mqttx)](https://goreportcard.com/report/github.com/darkit/mqttx)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/darkit/mqttx/blob/master/LICENSE)

## 🚀 项目简介

MQTTX 是一个为 Go 应用程序设计的高性能多会话 MQTT 客户端库。经过深度优化，提供了卓越的性能、简洁的 API 和强大的功能。

## ✨ 核心特性

### 🏗️ 架构优化
- **Builder 模式**: 流畅的 API 设计，简化配置过程
- **对象池技术**: 自动内存管理，28.5x 性能提升，0 内存分配
- **原子操作优化**: 4M+ 原子操作/秒，确保并发安全
- **统一错误处理**: 结构化错误类型，增强错误信息质量

### 🎯 功能特性
- **多会话管理**: 并发处理多个 MQTT 连接
- **消息转发系统**: 跨会话和跨主题的消息转发处理
- **自动重连机制**: 内置指数退避重连策略
- **TLS/SSL 支持**: 证书认证的安全通信
- **会话持久化**: 支持内存、文件和 Redis 存储
- **实时监控**: 详细的性能和健康指标

### 🔧 技术特性  
- **线程安全设计**: 所有操作都是并发安全的
- **性能监控**: 内置指标收集和性能分析
- **灵活配置**: 丰富的配置选项和调优参数
- **错误恢复**: 智能错误检测和恢复机制

## 安装方法

```bash
go get github.com/darkit/mqttx
```

## 🚀 快速开始

### 基础使用

```go
package main

import (
    "log"
    "time"
    "github.com/darkit/mqttx"
)

func main() {
    // 创建会话管理器
    manager := mqttx.NewSessionManager()
    defer manager.Close()

    // 使用 Builder 模式创建会话
    opts, err := mqttx.QuickConnect("生产设备", "broker.example.com:1883").
        Auth("username", "password").
        KeepAlive(60).
        AutoReconnect().
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 添加会话并连接
    if err := manager.AddSession(opts); err != nil {
        log.Fatal(err)
    }

    if err := manager.ConnectAll(); err != nil {
        log.Fatal(err)
    }

    // 等待连接完成
    if err := manager.WaitForAllSessions(30 * time.Second); err != nil {
        log.Printf("连接警告: %v", err)
    }

    // 发布和订阅消息
    session, _ := manager.GetSession("生产设备")
    
    // 订阅主题
    handler := func(topic string, payload []byte) {
        log.Printf("收到消息: %s = %s", topic, string(payload))
    }
    session.Subscribe("sensors/+/temperature", 1, handler)
    
    // 发布消息
    session.Publish("sensors/room1/temperature", []byte("23.5"), 1, false)
    
    select {} // 保持运行
}
```

## 📚 核心概念

### Builder 模式

MQTTX 提供了流畅的 API 来简化配置：

```go
// 快速连接
opts, err := mqttx.QuickConnect("session-name", "localhost:1883").Build()

// 安全连接
opts, err := mqttx.SecureConnect("secure-session", "ssl://broker:8883", "/path/to/ca.crt").
    Auth("user", "pass").
    KeepAlive(60).
    Build()

// 复杂配置
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

### 消息转发

自动在会话间转发消息：

```go
// 创建转发器
config, err := mqttx.NewForwarderBuilder("sensor-forwarder").
    Source("sensor-session", "sensors/+/temperature").
    Target("storage-session").
    QoS(1).
    MapTopic("sensors/room1/temperature", "storage/room1/temp").
    Build()

forwarder, err := mqttx.NewForwarder(config, manager)
forwarder.Start()
```

### 错误处理

统一的错误处理机制：

```go
// 检查错误类型
if mqttx.IsTemporary(err) {
    // 临时错误，可重试
    log.Printf("临时错误: %v", err)
} else if mqttx.IsTimeout(err) {
    // 超时错误
    log.Printf("超时错误: %v", err)
}

// 创建自定义错误
err := mqttx.NewConnectionError("连接失败", originalErr).
    WithSession("my-session").
    WithContext("retry_count", 3)
```

## 📊 性能指标

MQTTX 在标准硬件上的性能表现：

- **消息吞吐量**: 100K+ 消息/秒
- **指标操作**: 4M+ 原子操作/秒
- **对象池优化**: 28.5x 性能提升
- **内存效率**: 每指标对象 < 5 字节
- **转发器性能**: 500K+ 生命周期/秒

### 性能监控

```go
// 全局指标
globalMetrics := manager.GetMetrics()
log.Printf("总消息数: %d, 错误数: %d", 
    globalMetrics.TotalMessages, globalMetrics.ErrorCount)

// 会话指标  
sessionMetrics := session.GetMetrics()
log.Printf("已发送: %d, 已接收: %d", 
    sessionMetrics.MessagesSent, sessionMetrics.MessagesReceived)

// 转发器指标
forwarderMetrics := forwarder.GetMetrics()
log.Printf("已转发: %d, 已丢弃: %d", 
    forwarderMetrics.MessagesSent, forwarderMetrics.MessagesDropped)
```

## 核心组件

### 会话管理器

会话管理器（`Manager`）是处理多个 MQTT 会话的核心组件：

```go
// 创建新的管理器
m := manager.NewSessionManager()

// 添加会话
err := m.AddSession(&manager.Options{...})

// 获取会话状态
status := m.GetAllSessionsStatus()

// 移除会话
err := m.RemoveSession("会话名称")

// 列出所有会话
sessions := m.ListSessions()
```

### 连接管理

管理器提供连接等待机制，确保会话在操作前准备就绪：

```go
// 等待特定会话连接
err := m.AddSession(opts)
if err != nil {
    log.Fatal(err)
}

// 等待会话就绪，超时时间30秒
if err := m.WaitForSession("生产设备", 30*time.Second); err != nil {
    log.Fatal(err)
}

// 或等待所有会话就绪
if err := m.WaitForAllSessions(30*time.Second); err != nil {
    log.Fatal(err)
}
```

### 消息处理

提供四种灵活的消息处理模式：

1. **Handle** - 全局回调处理：
```go
route := m.Handle("主题/#", func(msg *manager.Message) {
    log.Printf("收到消息：%s", msg.PayloadString())
})
defer route.Stop()
```

2. **HandleTo** - 特定会话回调处理：
```go
route, err := m.HandleTo("会话名称", "主题/#", func(msg *manager.Message) {
    log.Printf("会话收到消息：%s", msg.PayloadString())
})
defer route.Stop()
```

3. **Listen** - 通道消息接收：
```go
messages, route := m.Listen("主题/#")
go func() {
    for msg := range messages {
        log.Printf("收到消息：%s", msg.PayloadString())
    }
}()
defer route.Stop()
```

4. **ListenTo** - 特定会话通道接收：
```go
messages, route, err := m.ListenTo("会话名称", "主题/#")
go func() {
    for msg := range messages {
        log.Printf("收到消息：%s", msg.PayloadString())
    }
}()
defer route.Stop()
```

### 消息转发器

消息转发器允许在不同会话和主题之间自动转发消息，支持过滤、转换和元数据注入：

```go
// 创建转发器管理器
forwarderManager := mqttx.NewForwarderManager(manager)

// 配置转发器
forwarderConfig := mqttx.ForwarderConfig{
    Name:           "温度转发器",
    SourceSessions: []string{"源会话1", "源会话2"},
    SourceTopics:   []string{"sensors/+/temperature"},
    TargetSession:  "目标会话",
    TopicMapping:   map[string]string{
        "sensors/living-room/temperature": "processed/temperature/living-room",
    },
    QoS:            1,
    Metadata: map[string]interface{}{
        "forwarded_by": "温度转发器",
        "timestamp":    time.Now().Unix(),
    },
    Enabled:        true,
}

// 添加并启动转发器
forwarder, err := forwarderManager.AddForwarder(forwarderConfig)
if err != nil {
    log.Fatal(err)
}

// 获取转发器指标
metrics := forwarder.GetMetrics()
log.Printf("已转发消息: %d", metrics["messages_forwarded"])

// 停止所有转发器
forwarderManager.StopAll()
```

转发器支持以下功能：

1. **多源转发** - 从多个会话订阅消息
2. **主题映射** - 将源主题映射到不同的目标主题
3. **消息过滤** - 基于主题或内容过滤消息
4. **消息转换** - 在转发前转换消息内容
5. **元数据注入** - 向转发的消息添加元数据
6. **性能指标** - 提供详细的转发统计信息

### 事件系统

监控会话生命周期和状态变化，提供详细的事件信息：

```go
// 监控连接状态
m.OnEvent("session_ready", func(event manager.Event) {
    log.Printf("会话 %s 已准备就绪", event.Session)
})

// 监控状态变化
m.OnEvent("session_state_changed", func(event manager.Event) {
    stateData := event.Data.(map[string]interface{})
    log.Printf("会话 %s 状态从 %v 变更为 %v",
        event.Session,
        stateData["old_state"],
        stateData["new_state"])
})
```

可用事件：
- `session_connecting` - 会话正在连接中
- `session_connected` - 会话已成功连接
- `session_ready` - 会话已准备就绪
- `session_disconnected` - 会话已断开连接（包含错误信息）
- `session_reconnecting` - 会话正在尝试重连
- `session_added` - 新会话已添加到管理器
- `session_removed` - 会话已从管理器中移除
- `session_state_changed` - 会话状态已发生变化

事件数据结构：
```go
type Event struct {
    Type      string      // 事件类型
    Session   string      // 会话名称
    Data      interface{} // 附加事件数据
    Timestamp time.Time   // 事件时间戳
}
```

常见事件数据内容：
- `session_connected`：连接详情
- `session_disconnected`：错误信息（如果有）
- `session_state_changed`：包含 "old_state" 和 "new_state" 的映射
- `session_reconnecting`：重连尝试次数
- `session_ready`：会话配置摘要

### 高级配置

#### TLS 安全配置

```go
opts := &manager.Options{
    Name:     "安全会话",
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

#### 性能调优

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

#### 会话持久化

```go
opts := &manager.Options{
    ConnectProps: &manager.ConnectProps{
        PersistentSession: true,
        ResumeSubs:       true,
    },
}
```

#### 指标收集

监控会话和管理器性能：

```go
// 获取管理器级别的指标
metrics := m.GetMetrics()

// 获取特定会话的指标
session, _ := m.GetSession("会话名称")
sessionMetrics := session.GetMetrics()

// 获取所有转发器的指标
forwarderMetrics := forwarderManager.GetAllMetrics()
```

##### Prometheus 集成

支持通过 HTTP 端点暴露 Prometheus 格式的指标：

```go
// 创建 HTTP 服务暴露 Prometheus 指标
go func() {
    promExporter := manager.NewPrometheusExporter("mqtt")
    
    http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        var output strings.Builder
        
        // 收集管理器指标
        metrics := m.GetMetrics()
        output.WriteString(promExporter.Export(metrics))
        
        // 收集所有会话指标
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

在 Prometheus 配置中添加抓取目标：

```yaml
scrape_configs:
  - job_name: 'mqtt_metrics'
    static_configs:
      - targets: ['localhost:2112']
    scrape_interval: 15s
```

可用的 Prometheus 指标包括：

消息指标：
- `mqtt_session_messages_sent_total` - 发送的消息总数
- `mqtt_session_messages_received_total` - 接收的消息总数
- `mqtt_session_bytes_sent_total` - 发送的字节总数
- `mqtt_session_bytes_received_total` - 接收的字节总数
- `mqtt_session_message_rate` - 当前每秒消息数
- `mqtt_session_avg_message_rate` - 启动以来的平均每秒消息数
- `mqtt_session_bytes_rate` - 每秒字节数

状态指标：
- `mqtt_session_connected` - 会话连接状态（0/1）
- `mqtt_session_status` - 会话状态码
- `mqtt_session_subscriptions` - 活跃订阅数量
- `mqtt_session_errors_total` - 错误总数
- `mqtt_session_reconnects_total` - 重连次数

时间戳指标：
- `mqtt_session_last_message_timestamp_seconds` - 最后消息的 Unix 时间戳
- `mqtt_session_last_error_timestamp_seconds` - 最后错误的 Unix 时间戳

会话属性：
- `mqtt_session_persistent` - 持久会话标志（0/1）
- `mqtt_session_clean_session` - 清理会话标志（0/1）
- `mqtt_session_auto_reconnect` - 自动重连标志（0/1）

所有指标都包含 `session="会话名称"` 标签，便于按会话进行过滤和聚合。

## 最佳实践

1. **资源管理**
   - 始终使用 `defer route.Stop()` 清理订阅
   - 实现适当的错误处理
   - 使用有意义的会话名称和客户端 ID

2. **性能优化**
   - 根据使用场景配置适当的缓冲区大小
   - 尽可能使用特定会话的订阅（`HandleTo`/`ListenTo`）
   - 监控指标以识别性能瓶颈
   - 对比当前和平均消息速率以识别流量模式
   - 利用指标数据进行容量规划和性能调优

3. **可靠性**
   - 在生产环境中启用自动重连
   - 实现适当的错误处理和重试机制
   - 使用适合场景的 QoS 级别

4. **安全性**
   - 在生产环境中启用 TLS
   - 使用强客户端认证
   - 定期轮换凭证

5. **转发器使用**
   - 为转发器设置合理的缓冲区大小，避免消息丢失
   - 使用过滤器减少不必要的消息转发
   - 监控转发器指标，及时发现问题
   - 为复杂场景设计合理的主题映射策略

## 🔧 高级功能

### 会话持久化

支持多种存储后端：

```go
// 内存存储（默认，最快）
opts := mqttx.NewSessionBuilder("memory-session").
    Broker("localhost:1883").
    Build()

// 文件存储
opts := mqttx.NewSessionBuilder("file-session").
    Broker("localhost:1883").
    FileStorage("/var/lib/mqttx").
    Build()

// Redis 存储  
opts := mqttx.NewSessionBuilder("redis-session").
    Broker("localhost:1883").
    RedisStorage("localhost:6379").
    RedisAuth("user", "pass", 1).
    Build()
```

### 性能调优

```go
// 高性能配置
opts := mqttx.NewSessionBuilder("high-perf").
    Broker("localhost:1883").
    Performance(32, 10000).      // 32KB缓冲区, 10K pending消息
    MessageChannelSize(2000).    // 2K消息通道
    KeepAlive(300).             // 5分钟保活
    Timeouts(10, 5).            // 10s连接, 5s写入超时
    Build()
```

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 运行基准测试
go test -bench=. -benchmem

# 运行并发安全测试
go test -race ./...

# 性能测试
go test -run TestPerformanceImprovement -v
```

## 📖 文档

- [GoDoc](https://pkg.go.dev/github.com/darkit/mqttx) - 完整的 API 参考

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！请确保：

1. 代码通过所有测试
2. 遵循现有的代码风格
3. 添加必要的测试用例
4. 更新相关文档

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)。

## 🏆 致谢

感谢以下项目的启发和支持：

- [Eclipse Paho MQTT Go Client](https://github.com/eclipse/paho.mqtt.golang)
- [Go Redis](https://github.com/redis/go-redis)

---

**MQTTX** - 让 MQTT 客户端开发更简单、更高效！