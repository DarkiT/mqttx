# TLS证书目录

此目录用于存放TLS证书文件，用于MQTTX增强TLS配置示例。

## 所需文件

- `ca.crt` - CA根证书
- `client.crt` - 客户端证书
- `client.key` - 客户端私钥

## 生成自签名证书

以下是使用OpenSSL生成自签名证书的简单步骤：

### 1. 生成CA证书

```bash
# 生成CA私钥
openssl genrsa -out ca.key 2048

# 生成CA证书
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=MQTTX/OU=Test/CN=MQTTX CA"
```

### 2. 生成服务器证书

```bash
# 生成服务器私钥
openssl genrsa -out server.key 2048

# 生成证书签名请求
openssl req -new -key server.key -out server.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=MQTTX/OU=Server/CN=localhost"

# 使用CA证书签名服务器证书
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt -days 365 -sha256
```

### 3. 生成客户端证书

```bash
# 生成客户端私钥
openssl genrsa -out client.key 2048

# 生成证书签名请求
openssl req -new -key client.key -out client.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=MQTTX/OU=Client/CN=mqttx-client"

# 使用CA证书签名客户端证书
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out client.crt -days 365 -sha256
```

### 4. 生成带密码的客户端私钥（可选）

```bash
# 生成带密码的客户端私钥
openssl genrsa -des3 -out client.key 2048
# 输入密码: client-key-password
```

## 配置MQTT Broker

请确保您的MQTT Broker已正确配置TLS支持。以下是Mosquitto的示例配置：

```
listener 8883
cafile /path/to/ca.crt
certfile /path/to/server.crt
keyfile /path/to/server.key
require_certificate true
use_identity_as_username true
```

## 注意事项

- 生产环境中应使用受信任的CA签发的证书
- 确保证书文件的权限设置正确，私钥应该只有所有者可读
- 定期更新证书以保持安全性 