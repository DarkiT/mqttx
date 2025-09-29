package mqttx

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionState_Serialization(t *testing.T) {
	// 创建测试数据
	now := time.Now()
	state := &SessionState{
		Topics: []TopicSubscription{
			{Topic: "test/topic1", QoS: 1},
			{Topic: "test/topic2", QoS: 0},
		},
		Messages: []*Message{
			{Topic: "test/topic1", Payload: []byte("message1"), QoS: 1},
			{Topic: "test/topic2", Payload: []byte("message2"), QoS: 0},
		},
		QoSMessages: map[uint16]*Message{
			1: {Topic: "test/qos", Payload: []byte("qos message"), QoS: 1, MessageID: 1},
		},
		RetainedMessages: map[string]*Message{
			"test/retained": {Topic: "test/retained", Payload: []byte("retained"), Retained: true},
		},
		LastSequence:     42,
		LastConnected:    now,
		LastDisconnected: now.Add(-1 * time.Hour),
		ClientID:         "test-client",
		SessionExpiry:    now.Add(24 * time.Hour),
		Version:          1,
	}

	// 序列化
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Failed to marshal state: %v", err)
	}

	// 反序列化
	var loadedState SessionState
	err = json.Unmarshal(data, &loadedState)
	if err != nil {
		t.Fatalf("Failed to unmarshal state: %v", err)
	}

	// 验证字段
	if len(loadedState.Topics) != len(state.Topics) {
		t.Errorf("Topics count mismatch: got %d, want %d", len(loadedState.Topics), len(state.Topics))
	}

	if len(loadedState.Messages) != len(state.Messages) {
		t.Errorf("Messages count mismatch: got %d, want %d", len(loadedState.Messages), len(state.Messages))
	}

	if len(loadedState.QoSMessages) != len(state.QoSMessages) {
		t.Errorf("QoSMessages count mismatch: got %d, want %d", len(loadedState.QoSMessages), len(state.QoSMessages))
	}

	if len(loadedState.RetainedMessages) != len(state.RetainedMessages) {
		t.Errorf("RetainedMessages count mismatch: got %d, want %d", len(loadedState.RetainedMessages), len(state.RetainedMessages))
	}

	if loadedState.LastSequence != state.LastSequence {
		t.Errorf("LastSequence mismatch: got %d, want %d", loadedState.LastSequence, state.LastSequence)
	}

	if loadedState.ClientID != state.ClientID {
		t.Errorf("ClientID mismatch: got %s, want %s", loadedState.ClientID, state.ClientID)
	}

	if loadedState.Version != state.Version {
		t.Errorf("Version mismatch: got %d, want %d", loadedState.Version, state.Version)
	}

	// 检查时间字段
	if !loadedState.LastConnected.Equal(state.LastConnected) {
		t.Errorf("LastConnected mismatch: got %v, want %v", loadedState.LastConnected, state.LastConnected)
	}

	if !loadedState.LastDisconnected.Equal(state.LastDisconnected) {
		t.Errorf("LastDisconnected mismatch: got %v, want %v", loadedState.LastDisconnected, state.LastDisconnected)
	}

	if !loadedState.SessionExpiry.Equal(state.SessionExpiry) {
		t.Errorf("SessionExpiry mismatch: got %v, want %v", loadedState.SessionExpiry, state.SessionExpiry)
	}
}

func TestSessionState_TopicSubscriptions(t *testing.T) {
	// 测试主题订阅
	topics := []TopicSubscription{
		{Topic: "test/topic1", QoS: 1},
		{Topic: "test/topic2", QoS: 0},
		{Topic: "test/+", QoS: 2},
		{Topic: "#", QoS: 0},
	}

	// 创建状态
	state := &SessionState{
		Topics: topics,
	}

	// 检查主题数量
	if len(state.Topics) != len(topics) {
		t.Errorf("Topics count mismatch: got %d, want %d", len(state.Topics), len(topics))
	}

	// 检查每个主题
	for i, topic := range topics {
		if state.Topics[i].Topic != topic.Topic {
			t.Errorf("Topic mismatch at index %d: got %s, want %s", i, state.Topics[i].Topic, topic.Topic)
		}
		if state.Topics[i].QoS != topic.QoS {
			t.Errorf("QoS mismatch at index %d: got %d, want %d", i, state.Topics[i].QoS, topic.QoS)
		}
	}
}

func TestSessionState_Messages(t *testing.T) {
	// 测试消息
	messages := []*Message{
		{Topic: "test/topic1", Payload: []byte("message1"), QoS: 1, Retained: false, Duplicate: false},
		{Topic: "test/topic2", Payload: []byte("message2"), QoS: 0, Retained: true, Duplicate: false},
		{Topic: "test/topic3", Payload: []byte("message3"), QoS: 2, Retained: false, Duplicate: true},
	}

	// 创建状态
	state := &SessionState{
		Messages: messages,
	}

	// 检查消息数量
	if len(state.Messages) != len(messages) {
		t.Errorf("Messages count mismatch: got %d, want %d", len(state.Messages), len(messages))
	}

	// 检查每个消息
	for i, msg := range messages {
		if state.Messages[i].Topic != msg.Topic {
			t.Errorf("Topic mismatch at index %d: got %s, want %s", i, state.Messages[i].Topic, msg.Topic)
		}
		if string(state.Messages[i].Payload) != string(msg.Payload) {
			t.Errorf("Payload mismatch at index %d: got %s, want %s", i, string(state.Messages[i].Payload), string(msg.Payload))
		}
		if state.Messages[i].QoS != msg.QoS {
			t.Errorf("QoS mismatch at index %d: got %d, want %d", i, state.Messages[i].QoS, msg.QoS)
		}
		if state.Messages[i].Retained != msg.Retained {
			t.Errorf("Retained mismatch at index %d: got %t, want %t", i, state.Messages[i].Retained, msg.Retained)
		}
		if state.Messages[i].Duplicate != msg.Duplicate {
			t.Errorf("Duplicate mismatch at index %d: got %t, want %t", i, state.Messages[i].Duplicate, msg.Duplicate)
		}
	}
}

func TestSessionState_QoSMessages(t *testing.T) {
	// 测试QoS消息
	qosMessages := map[uint16]*Message{
		1: {Topic: "test/qos1", Payload: []byte("qos1"), QoS: 1, MessageID: 1},
		2: {Topic: "test/qos2", Payload: []byte("qos2"), QoS: 2, MessageID: 2},
	}

	// 创建状态
	state := &SessionState{
		QoSMessages: qosMessages,
	}

	// 检查消息数量
	if len(state.QoSMessages) != len(qosMessages) {
		t.Errorf("QoSMessages count mismatch: got %d, want %d", len(state.QoSMessages), len(qosMessages))
	}

	// 检查每个消息
	for id, msg := range qosMessages {
		stateMsg, ok := state.QoSMessages[id]
		if !ok {
			t.Errorf("Message ID %d not found in state", id)
			continue
		}
		if stateMsg.Topic != msg.Topic {
			t.Errorf("Topic mismatch for message ID %d: got %s, want %s", id, stateMsg.Topic, msg.Topic)
		}
		if string(stateMsg.Payload) != string(msg.Payload) {
			t.Errorf("Payload mismatch for message ID %d: got %s, want %s", id, string(stateMsg.Payload), string(msg.Payload))
		}
		if stateMsg.QoS != msg.QoS {
			t.Errorf("QoS mismatch for message ID %d: got %d, want %d", id, stateMsg.QoS, msg.QoS)
		}
		if stateMsg.MessageID != msg.MessageID {
			t.Errorf("MessageID mismatch for message ID %d: got %d, want %d", id, stateMsg.MessageID, msg.MessageID)
		}
	}
}

func TestSessionState_RetainedMessages(t *testing.T) {
	// 测试保留消息
	retainedMessages := map[string]*Message{
		"test/retained1": {Topic: "test/retained1", Payload: []byte("retained1"), Retained: true},
		"test/retained2": {Topic: "test/retained2", Payload: []byte("retained2"), Retained: true},
	}

	// 创建状态
	state := &SessionState{
		RetainedMessages: retainedMessages,
	}

	// 检查消息数量
	if len(state.RetainedMessages) != len(retainedMessages) {
		t.Errorf("RetainedMessages count mismatch: got %d, want %d", len(state.RetainedMessages), len(retainedMessages))
	}

	// 检查每个消息
	for topic, msg := range retainedMessages {
		stateMsg, ok := state.RetainedMessages[topic]
		if !ok {
			t.Errorf("Topic %s not found in state", topic)
			continue
		}
		if stateMsg.Topic != msg.Topic {
			t.Errorf("Topic mismatch for topic %s: got %s, want %s", topic, stateMsg.Topic, msg.Topic)
		}
		if string(stateMsg.Payload) != string(msg.Payload) {
			t.Errorf("Payload mismatch for topic %s: got %s, want %s", topic, string(stateMsg.Payload), string(msg.Payload))
		}
		if !stateMsg.Retained {
			t.Errorf("Retained flag should be true for topic %s", topic)
		}
	}
}
