package mqttx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageFilters(t *testing.T) {
	// 测试主题过滤器
	t.Run("TopicFilter", func(t *testing.T) {
		filter, err := NewTopicFilter("sensor/+/temperature", false)
		assert.NoError(t, err)

		// 应该匹配
		msg1 := &Message{Topic: "sensor/kitchen/temperature"}
		assert.True(t, filter.Match(msg1))

		// 不应该匹配
		msg2 := &Message{Topic: "sensor/kitchen/humidity"}
		assert.False(t, filter.Match(msg2))

		// 使用正则表达式的过滤器
		regexFilter, err := NewTopicFilter("sensor/[0-9]+/data", true)
		assert.NoError(t, err)

		// 应该匹配
		msg3 := &Message{Topic: "sensor/123/data"}
		assert.True(t, regexFilter.Match(msg3))

		// 不应该匹配
		msg4 := &Message{Topic: "sensor/abc/data"}
		assert.False(t, regexFilter.Match(msg4))
	})

	// 测试负载过滤器
	t.Run("PayloadFilter", func(t *testing.T) {
		// 简单包含过滤器
		filter, err := NewPayloadFilter("temperature", false)
		assert.NoError(t, err)

		// 应该匹配
		msg1 := &Message{Payload: []byte(`{"temperature":25.5}`)}
		assert.True(t, filter.Match(msg1))

		// 不应该匹配
		msg2 := &Message{Payload: []byte(`{"humidity":60}`)}
		assert.False(t, filter.Match(msg2))

		// 空负载测试
		msg3 := &Message{Payload: nil}
		assert.False(t, filter.Match(msg3))
	})

	// 测试组合过滤器
	t.Run("CompositeFilter", func(t *testing.T) {
		topicFilter, _ := NewTopicFilter("device/+/data", false)
		payloadFilter, _ := NewPayloadFilter("temperature", false)

		// AND过滤器 - 两个条件都必须满足
		andFilter := NewAndFilter(topicFilter, payloadFilter)

		msg1 := &Message{
			Topic:   "device/123/data",
			Payload: []byte(`{"temperature":25.5}`),
		}
		assert.True(t, andFilter.Match(msg1), "应该匹配同时满足主题和负载条件的消息")

		msg2 := &Message{
			Topic:   "device/123/data",
			Payload: []byte(`{"humidity":60}`),
		}
		assert.False(t, andFilter.Match(msg2), "不应该匹配只满足主题但不满足负载条件的消息")

		// OR过滤器 - 任一条件满足即可
		orFilter := NewOrFilter(topicFilter, payloadFilter)

		assert.True(t, orFilter.Match(msg1), "应该匹配满足主题和负载条件的消息")
		assert.True(t, orFilter.Match(msg2), "应该匹配只满足主题条件的消息")

		msg3 := &Message{
			Topic:   "sensor/123/data",
			Payload: []byte(`{"temperature":25.5}`),
		}
		assert.True(t, orFilter.Match(msg3), "应该匹配只满足负载条件的消息")

		msg4 := &Message{
			Topic:   "sensor/123/data",
			Payload: []byte(`{"humidity":60}`),
		}
		assert.False(t, orFilter.Match(msg4), "不应该匹配两个条件都不满足的消息")
	})

	// 测试主题匹配函数
	t.Run("TopicMatch", func(t *testing.T) {
		tests := []struct {
			name     string
			filter   string
			topic    string
			expected bool
		}{
			{"精确匹配", "a/b/c", "a/b/c", true},
			{"不匹配", "a/b/c", "a/b/d", false},
			{"单层级通配符", "a/+/c", "a/b/c", true},
			{"单层级通配符不匹配", "a/+/c", "a/b/c/d", false},
			{"多层级通配符", "a/#", "a/b/c", true},
			{"多层级通配符根", "#", "a/b/c", true},
			{"混合通配符", "a/+/c/#", "a/b/c/d/e", true},
			{"混合通配符不匹配", "a/+/c/#", "a/b/d/e", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := TopicMatch(tt.filter, tt.topic)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}
