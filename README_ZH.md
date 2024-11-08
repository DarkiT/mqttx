# MQTT 会话管理器

[![PkgGoDev](https://pkg.go.dev/badge/github.com/darkit/mqttx.svg)](https://pkg.go.dev/github.com/darkit/mqttx)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkit/mqttx)](https://goreportcard.com/report/github.com/darkit/mqttx)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/darkit/mqttx/blob/master/LICENSE)

## 简介

一个强大的 Go 语言多会话 MQTT 管理器，提供多个 MQTT 连接的并发管理功能。专注于可靠性、灵活性和性能。

## 核心特性

- 🔄 多会话管理：并发处理多个 MQTT 连接
- 🔌 自动重连：内置可配置的重连机制
- 🔒 TLS/SSL 支持：支持基于证书的安全通信
- 📨 灵活的消息路由：多种消息处理模式（同步/异步）
- 📊 指标收集：详细的性能和健康状态指标
- 💾 会话持久化：可选的会话状态持久化
- 🎯 事件系统：完整的事件通知系统
- 🛡️ 线程安全：保证并发操作安全

## 安装方法

```bash
go get github.com/darkit/mqttx
```

## 快速开始

```go
func main() {
    // 创建会话管理器
    m := manager.NewSessionManager()

    // 配置会话选项
    opts := &manager.Options{
        Name:     "生产设备",
        Brokers:  []string{"tcp://broker.example.com:1883"},
        ClientID: "device-001",
        ConnectProps: &manager.ConnectProps{
            KeepAlive:     60,
            CleanSession:  true,
            AutoReconnect: true,
        },
    }

    // 添加会话并等待就绪
    if err := m.AddSession(opts); err != nil {
        log.Fatal(err)
    }

    // 等待会话就绪
    if err := m.WaitForSession("生产设备", 30*time.Second); err != nil {
        log.Fatal(err)
    }

    // 现在可以安全地订阅和发布消息
    route := m.Handle("sensors/+/temperature", func(msg *manager.Message) {
        log.Printf("温度读数：%s", msg.PayloadString())
    })
    defer route.Stop()

    err := m.PublishTo("生产设备", "sensors/room1/temperature", []byte("23.5"), 1)
    if err != nil {
        log.Printf("发布失败：%v", err)
    }

    select {} // 保持运行
}
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

## 开源协议

MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情