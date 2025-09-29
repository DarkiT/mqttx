# MQTT 5.0 增强认证示例

本示例展示了如何使用MQTTX库的MQTT 5.0增强认证功能，包括JWT认证和OAuth 2.0认证。

## 功能特点

- MQTT 5.0协议支持
- 增强认证机制
  - JWT认证
  - OAuth 2.0认证
- 用户属性
- 共享订阅
- 会话过期控制

## 前提条件

- Go 1.21或更高版本
- 支持MQTT 5.0的MQTT代理服务器（如EMQX、Mosquitto 2.0+等）

## 运行示例

1. 确保MQTT代理服务器已启动并支持MQTT 5.0
2. 在本目录下运行示例程序：

```bash
go run main.go
```

## 示例说明

本示例创建了两个MQTT会话：

1. **JWT认证会话**：使用JWT令牌进行认证
2. **OAuth 2.0认证会话**：使用OAuth 2.0访问令牌进行认证

两个会话都配置了MQTT 5.0特性：

- 会话过期时间设置为30分钟
- 添加了自定义用户属性
- 使用共享订阅功能

## 注意事项

- 本示例中使用的JWT令牌和OAuth令牌仅用于演示，实际应用中应使用有效的令牌
- 默认连接到本地MQTT代理服务器（localhost:1883），如需修改，请更改代码中的Brokers设置
- 服务端需要支持相应的认证方法，否则连接可能会失败

## 更多信息

有关MQTT 5.0特性的更多信息，请参考以下资源：

- [MQTT 5.0规范](https://docs.oasis-open.org/mqtt/mqtt/v5.0/mqtt-v5.0.html)
- [MQTTX库文档](https://github.com/darkit/mqttx) 