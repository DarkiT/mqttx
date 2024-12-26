package mqttx

// newEventManager 创建新的事件管理器
func newEventManager() *EventManager {
	return &EventManager{
		handlers: make(map[string][]EventHandler),
	}
}

// emit 发送事件到所有注册的处理函数
func (em *EventManager) emit(event Event) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if handlers, ok := em.handlers[event.Type]; ok {
		for _, handler := range handlers {
			handler(event)
		}
	}
}

// on 注册事件处理函数
func (em *EventManager) on(eventType string, handler EventHandler) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.handlers[eventType] = append(em.handlers[eventType], handler)
}
