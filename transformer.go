package mqttx

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// MessageTransformer 消息转换器接口
// 用于对消息进行转换处理
type MessageTransformer interface {
	// Transform 转换消息，返回转换后的消息
	Transform(message *Message) (*Message, error)
	// GetDescription 获取转换器的描述
	GetDescription() string
}

// TopicRewriteTransformer 主题重写转换器
// 用于将消息的主题按照一定规则进行重写
type TopicRewriteTransformer struct {
	pattern     string         // 匹配模式，可以是精确匹配或正则表达式
	replacement string         // 替换模式
	isRegex     bool           // 是否为正则表达式模式
	regexFilter *regexp.Regexp // 编译好的正则表达式
}

// NewTopicRewriteTransformer 创建主题重写转换器
// 如果isRegex=true，pattern将被视为正则表达式，replacement可以包含正则表达式捕获组引用
// 例如：pattern="sensor/([^/]+)/temp", replacement="device/$1/temperature"
func NewTopicRewriteTransformer(pattern, replacement string, isRegex bool) (*TopicRewriteTransformer, error) {
	transformer := &TopicRewriteTransformer{
		pattern:     pattern,
		replacement: replacement,
		isRegex:     isRegex,
	}

	if isRegex {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, wrapError(err, "invalid regex pattern")
		}
		transformer.regexFilter = regex
	}

	return transformer, nil
}

// Transform 转换消息的主题
func (t *TopicRewriteTransformer) Transform(message *Message) (*Message, error) {
	if message == nil {
		return nil, nil
	}

	// 创建消息副本
	result := *message

	// 使用正则表达式进行主题重写
	if t.isRegex {
		result.Topic = t.regexFilter.ReplaceAllString(message.Topic, t.replacement)
	} else {
		// 简单字符串替换
		if message.Topic == t.pattern {
			result.Topic = t.replacement
		}
	}

	return &result, nil
}

// GetDescription 获取转换器的描述
func (t *TopicRewriteTransformer) GetDescription() string {
	return fmt.Sprintf("TopicRewrite [%s] -> [%s] (regex:%v)", t.pattern, t.replacement, t.isRegex)
}

// PayloadTransformer 负载转换器
// 用于对消息负载进行转换
type PayloadTransformer struct {
	transformFunc func([]byte) ([]byte, error) // 转换函数
	description   string                       // 转换器描述
}

// NewPayloadTransformer 创建自定义负载转换器
func NewPayloadTransformer(description string, transformFunc func([]byte) ([]byte, error)) *PayloadTransformer {
	return &PayloadTransformer{
		transformFunc: transformFunc,
		description:   description,
	}
}

// Transform 转换消息的负载
func (t *PayloadTransformer) Transform(message *Message) (*Message, error) {
	if message == nil || t.transformFunc == nil {
		return message, nil
	}

	// 创建消息副本
	result := *message

	// 转换负载
	newPayload, err := t.transformFunc(message.Payload)
	if err != nil {
		return nil, wrapError(err, "failed to transform payload")
	}

	// 如果转换函数返回nil，表示要过滤掉这条消息
	if newPayload == nil {
		return nil, nil
	}

	result.Payload = newPayload
	return &result, nil
}

// GetDescription 获取转换器的描述
func (t *PayloadTransformer) GetDescription() string {
	return t.description
}

// JsonPathTransformer JSON路径转换器
// 用于对JSON格式的负载进行特定字段的修改
type JsonPathTransformer struct {
	path        string      // JSON路径表达式
	value       interface{} // 要设置的值
	description string      // 转换器描述
}

// NewJsonPathTransformer 创建JSON路径转换器
// path格式为简单的点分路径，例如 "device.temperature.value"
func NewJsonPathTransformer(path string, value interface{}) *JsonPathTransformer {
	return &JsonPathTransformer{
		path:        path,
		value:       value,
		description: fmt.Sprintf("JsonPath [%s]", path),
	}
}

// Transform 转换JSON负载中的指定字段
func (t *JsonPathTransformer) Transform(message *Message) (*Message, error) {
	if message == nil || len(message.Payload) == 0 {
		return message, nil
	}

	// 尝试解析JSON负载
	var jsonData map[string]interface{}
	if err := json.Unmarshal(message.Payload, &jsonData); err != nil {
		return nil, wrapError(err, "failed to parse JSON payload")
	}

	// 修改JSON数据
	if err := setJsonValue(jsonData, t.path, t.value); err != nil {
		return nil, wrapError(err, "failed to set JSON value")
	}

	// 重新编码JSON
	newPayload, err := json.Marshal(jsonData)
	if err != nil {
		return nil, wrapError(err, "failed to encode JSON payload")
	}

	// 创建消息副本
	result := *message
	result.Payload = newPayload

	return &result, nil
}

// GetDescription 获取转换器的描述
func (t *JsonPathTransformer) GetDescription() string {
	return t.description
}

// CompositeTransformer 组合转换器
// 可以将多个转换器组合在一起，按顺序应用
type CompositeTransformer struct {
	transformers []MessageTransformer
	description  string
}

// NewCompositeTransformer 创建组合转换器
func NewCompositeTransformer(description string, transformers ...MessageTransformer) *CompositeTransformer {
	return &CompositeTransformer{
		transformers: transformers,
		description:  description,
	}
}

// Transform 按顺序应用所有转换器
func (t *CompositeTransformer) Transform(message *Message) (*Message, error) {
	if message == nil || len(t.transformers) == 0 {
		return message, nil
	}

	current := message
	var err error

	// 按顺序应用每个转换器
	for _, transformer := range t.transformers {
		current, err = transformer.Transform(current)
		if err != nil {
			return nil, wrapError(err, "transformer error")
		}
		if current == nil {
			return nil, nil
		}
	}

	return current, nil
}

// GetDescription 获取转换器的描述
func (t *CompositeTransformer) GetDescription() string {
	if len(t.transformers) == 0 {
		return t.description + "(empty)"
	}

	descriptions := make([]string, len(t.transformers))
	for i, transformer := range t.transformers {
		descriptions[i] = transformer.GetDescription()
	}

	return t.description + "(" + strings.Join(descriptions, " -> ") + ")"
}

// setJsonValue 在JSON对象中设置指定路径的值
// path格式为点分路径，例如 "device.temperature.value"
func setJsonValue(jsonData map[string]interface{}, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("invalid JSON path")
	}

	// 处理嵌套路径
	current := jsonData
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}

		// 如果中间节点不存在，则创建
		next, ok := current[part]
		if !ok {
			next = make(map[string]interface{})
			current[part] = next
		}

		// 确保中间节点是map类型
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			// 如果不是map，则替换为map
			nextMap = make(map[string]interface{})
			current[part] = nextMap
		}

		current = nextMap
	}

	// 设置最终值
	current[parts[len(parts)-1]] = value
	return nil
}
