package mqttx

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPayloadTransformerAdvanced(t *testing.T) {
	// 测试基本转换
	t.Run("BasicTransform", func(t *testing.T) {
		// 创建一个简单的转换器，将JSON中的temperature字段值加10
		transformer := NewPayloadTransformer("Add 10 to temperature", func(payload []byte) ([]byte, error) {
			var data map[string]interface{}
			if err := json.Unmarshal(payload, &data); err != nil {
				return nil, err
			}

			if temp, ok := data["temperature"].(float64); ok {
				data["temperature"] = temp + 10
			}

			return json.Marshal(data)
		})

		original := []byte(`{"device":"sensor","temperature":25.5}`)
		msg := &Message{Payload: original}

		result, err := transformer.Transform(msg)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		var resultData map[string]interface{}
		err = json.Unmarshal(result.Payload, &resultData)
		assert.NoError(t, err)
		assert.Equal(t, 35.5, resultData["temperature"])
	})

	// 测试过滤功能（返回nil）
	t.Run("FilterMessage", func(t *testing.T) {
		// 创建一个过滤器，只保留温度高于30度的消息
		transformer := NewPayloadTransformer("Filter low temperature", func(payload []byte) ([]byte, error) {
			var data map[string]interface{}
			if err := json.Unmarshal(payload, &data); err != nil {
				return nil, err
			}

			if temp, ok := data["temperature"].(float64); ok && temp <= 30 {
				return nil, nil // 返回nil表示过滤掉这条消息
			}

			return payload, nil
		})

		// 应该被过滤掉的消息
		msg1 := &Message{Payload: []byte(`{"device":"sensor","temperature":25.5}`)}
		result1, err := transformer.Transform(msg1)
		assert.NoError(t, err)
		assert.Nil(t, result1)

		// 应该保留的消息
		msg2 := &Message{Payload: []byte(`{"device":"sensor","temperature":35.5}`)}
		result2, err := transformer.Transform(msg2)
		assert.NoError(t, err)
		assert.NotNil(t, result2)
	})
}

func TestJsonPathTransformerAdvanced(t *testing.T) {
	// 测试嵌套路径
	original := []byte(`{"device":"sensor","reading":{"temperature":25.5,"humidity":60}}`)

	nestedTransformer := NewJsonPathTransformer("reading.temperature", 30.0)
	msg := &Message{Payload: original}

	nestedResult, err := nestedTransformer.Transform(msg)
	assert.NoError(t, err)

	var nestedData map[string]interface{}
	json.Unmarshal(nestedResult.Payload, &nestedData)

	reading := nestedData["reading"].(map[string]interface{})
	assert.Equal(t, 30.0, reading["temperature"])
}

func TestCompositeTransformerAdvanced(t *testing.T) {
	// 测试转换器链中的过滤功能
	topicTransformer, err := NewTopicRewriteTransformer(
		"sensor/([^/]+)/temperature",
		"device/$1/temp",
		true,
	)
	assert.NoError(t, err)

	filterTransformer := NewPayloadTransformer("Filter low values", func(payload []byte) ([]byte, error) {
		var data map[string]interface{}
		if err := json.Unmarshal(payload, &data); err != nil {
			return nil, err
		}

		if val, ok := data["value"].(float64); ok && val < 20 {
			return nil, nil // 过滤掉低于20的值
		}

		return payload, nil
	})

	// 创建包含过滤器的组合转换器
	filterComposite := NewCompositeTransformer("Filter and Process",
		filterTransformer, topicTransformer)

	// 应该被过滤掉的消息
	lowValueMsg := &Message{
		Topic:   "sensor/kitchen/temperature",
		Payload: []byte(`{"device":"sensor","value":15.5}`),
	}

	filteredResult, err := filterComposite.Transform(lowValueMsg)
	assert.NoError(t, err)
	assert.Nil(t, filteredResult, "低值消息应该被过滤掉")

	// 不应该被过滤掉的消息
	highValueMsg := &Message{
		Topic:   "sensor/kitchen/temperature",
		Payload: []byte(`{"device":"sensor","value":25.5}`),
	}

	passedResult, err := filterComposite.Transform(highValueMsg)
	assert.NoError(t, err)
	assert.NotNil(t, passedResult, "高值消息不应该被过滤掉")
	assert.Equal(t, "device/kitchen/temp", passedResult.Topic)
}
