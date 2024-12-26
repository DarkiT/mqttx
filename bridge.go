package mqttx

import (
	"fmt"
	"sync/atomic"
	"time"
)

// CreateBridge 创建新的MQTT网桥
func (m *Manager) CreateBridge(config *BridgeConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查网桥是否已存在
	if _, exists := m.bridges[config.Name]; exists {
		return fmt.Errorf("bridge %s already exists", config.Name)
	}

	// 检查源会话和目标会话是否存在并已连接
	source, exists := m.sessions[config.SourceSession]
	if !exists {
		return fmt.Errorf("source session %s not found", config.SourceSession)
	}
	if !source.IsConnected() {
		return fmt.Errorf("source session %s is not connected", config.SourceSession)
	}

	target, exists := m.sessions[config.TargetSession]
	if !exists {
		return fmt.Errorf("target session %s not found", config.TargetSession)
	}
	if !target.IsConnected() {
		return fmt.Errorf("target session %s is not connected", config.TargetSession)
	}

	// 创建网桥信息
	bridgeInfo := &BridgeInfo{
		Name:          config.Name,
		SourceSession: config.SourceSession,
		TargetSession: config.TargetSession,
		Routes:        make([]*BridgeRoute, 0),
	}

	// 设置初始状态
	atomic.StoreUint32(&bridgeInfo.Status, StateConnecting)

	// 为每个主题映射创建路由
	for sourceTopic, targetTopic := range config.TopicMappings {
		// 使用闭包保存主题变量
		if err := func(src, dst string) error {
			// 创建消息处理函数
			handler := MessageHandler(func(topic string, payload []byte) {
				// 检查目标会话连接状态
				if !target.IsConnected() {
					m.logger.Debug("Target session disconnected, waiting for reconnection",
						"bridge", config.Name,
						"target", config.TargetSession)
					// 等待重连，最多等待5秒
					for i := 0; i < 5; i++ {
						if target.IsConnected() {
							break
						}
						time.Sleep(time.Second)
					}
				}

				// 再次检查连接状态
				if !target.IsConnected() {
					m.logger.Error("Target session still disconnected, dropping message",
						"bridge", config.Name,
						"source_topic", src,
						"target_topic", dst)
					return
				}

				if err := target.Publish(dst, payload, config.QoS); err != nil {
					m.logger.Error("Failed to forward message",
						"bridge", config.Name,
						"source_topic", src,
						"target_topic", dst,
						"error", err)
					m.metrics.recordError(err)
				} else {
					m.logger.Debug("Message forwarded",
						"bridge", config.Name,
						"source_topic", src,
						"target_topic", dst,
						"size", len(payload))
					m.metrics.recordMessage(uint64(len(payload)))
				}
			})

			// 订阅源主题
			m.logger.Debug("Subscribing to source topic",
				"bridge", config.Name,
				"topic", src,
				"qos", config.QoS)

			if err := source.Subscribe(src, handler, config.QoS); err != nil {
				return fmt.Errorf("failed to subscribe to topic %s: %w", src, err)
			}

			// 创建路由记录
			route := &BridgeRoute{
				SourceTopic: src,
				TargetTopic: dst,
				Stop: func() {
					if err := source.Unsubscribe(src); err != nil {
						m.logger.Error("Failed to unsubscribe from topic",
							"bridge", config.Name,
							"topic", src,
							"error", err)
					}
				},
			}
			bridgeInfo.Routes = append(bridgeInfo.Routes, route)
			return nil
		}(sourceTopic, targetTopic); err != nil {
			// 如果出错，清理已创建的路由
			for _, r := range bridgeInfo.Routes {
				r.Stop()
			}
			return err
		}
	}

	// 更新状态为已连接
	atomic.StoreUint32(&bridgeInfo.Status, StateConnected)
	bridgeInfo.TopicCount = len(config.TopicMappings)
	m.bridges[config.Name] = bridgeInfo

	m.logger.Info("Bridge created successfully",
		"bridge", config.Name,
		"source", config.SourceSession,
		"target", config.TargetSession,
		"topics", bridgeInfo.TopicCount)

	// 发送事件
	m.events.emit(Event{
		Type:      "bridge_created",
		Data:      config.Name,
		Timestamp: time.Now(),
	})

	return nil
}

// RemoveBridge 移除网桥
func (m *Manager) RemoveBridge(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bridge, exists := m.bridges[name]
	if !exists {
		return fmt.Errorf("bridge %s not found", name)
	}

	// 更新状态为关闭中
	atomic.StoreUint32(&bridge.Status, StateClosed)

	// 停止所有路由
	for _, route := range bridge.Routes {
		route.Stop()
	}

	delete(m.bridges, name)

	// 发送事件
	m.events.emit(Event{
		Type:      "bridge_removed",
		Data:      name,
		Timestamp: time.Now(),
	})

	return nil
}

// ListBridges 列出所有网桥
func (m *Manager) ListBridges() []BridgeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bridges := make([]BridgeInfo, 0, len(m.bridges))
	for _, bridge := range m.bridges {
		bridges = append(bridges, *bridge)
	}
	return bridges
}

// GetBridge 获取网桥信息
func (m *Manager) GetBridge(name string) (*BridgeInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bridge, exists := m.bridges[name]
	if !exists {
		return nil, fmt.Errorf("bridge %s not found", name)
	}
	return bridge, nil
}
