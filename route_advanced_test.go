package mqttx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdvancedRoute(t *testing.T) {
	t.Skip("需要完整的Manager实现才能测试，跳过")
}

func TestTopicMatch(t *testing.T) {
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
}
