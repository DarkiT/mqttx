package mqttx

import (
	"regexp"
	"strings"
)

// MessageFilter 消息过滤器接口
// 用于根据指定的规则过滤消息
type MessageFilter interface {
	// Match 判断消息是否匹配过滤规则
	Match(message *Message) bool
	// GetDescription 获取过滤器的描述
	GetDescription() string
}

// TopicFilter 基于主题的消息过滤器
type TopicFilter struct {
	pattern     string // 主题匹配模式
	isRegex     bool   // 是否为正则表达式模式
	regexFilter *regexp.Regexp
}

// NewTopicFilter 创建基于主题的消息过滤器
// pattern可以是：
// 1. 精确匹配的主题名称（例如 "sensor/temperature"）
// 2. 包含通配符的模式（例如 "sensor/#" 或 "sensor/+/temperature"）
// 3. 正则表达式（使用isRegex=true）
func NewTopicFilter(pattern string, isRegex bool) (*TopicFilter, error) {
	filter := &TopicFilter{
		pattern: pattern,
		isRegex: isRegex,
	}

	if isRegex {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, wrapError(err, "invalid regex pattern")
		}
		filter.regexFilter = regex
	}

	return filter, nil
}

// Match 判断消息是否匹配主题过滤规则
func (f *TopicFilter) Match(message *Message) bool {
	if message == nil {
		return false
	}

	// 使用正则表达式匹配
	if f.isRegex {
		return f.regexFilter.MatchString(message.Topic)
	}

	// 使用MQTT主题匹配规则
	return TopicMatch(f.pattern, message.Topic)
}

// GetDescription 获取过滤器的描述
func (f *TopicFilter) GetDescription() string {
	if f.isRegex {
		return "RegexTopicFilter: " + f.pattern
	}
	return "TopicFilter: " + f.pattern
}

// PayloadFilter 基于消息负载的过滤器
type PayloadFilter struct {
	pattern     string
	isRegex     bool
	regexFilter *regexp.Regexp
}

// NewPayloadFilter 创建基于负载内容的消息过滤器
func NewPayloadFilter(pattern string, isRegex bool) (*PayloadFilter, error) {
	filter := &PayloadFilter{
		pattern: pattern,
		isRegex: isRegex,
	}

	if isRegex {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, wrapError(err, "invalid regex pattern")
		}
		filter.regexFilter = regex
	}

	return filter, nil
}

// Match 判断消息负载是否匹配过滤规则
func (f *PayloadFilter) Match(message *Message) bool {
	if message == nil || message.Payload == nil {
		return false
	}

	payload := string(message.Payload)

	// 使用正则表达式匹配
	if f.isRegex {
		return f.regexFilter.MatchString(payload)
	}

	// 使用简单包含匹配
	return strings.Contains(payload, f.pattern)
}

// GetDescription 获取过滤器的描述
func (f *PayloadFilter) GetDescription() string {
	if f.isRegex {
		return "RegexPayloadFilter: " + f.pattern
	}
	return "PayloadFilter: " + f.pattern
}

// CompositeFilter 组合过滤器，可以组合多个过滤器形成复杂的过滤规则
type CompositeFilter struct {
	filters     []MessageFilter
	isAnd       bool // true表示所有过滤器都必须匹配，false表示任一过滤器匹配即可
	description string
}

// NewAndFilter 创建一个AND复合过滤器，所有子过滤器都必须匹配
func NewAndFilter(filters ...MessageFilter) *CompositeFilter {
	return &CompositeFilter{
		filters:     filters,
		isAnd:       true,
		description: "AndFilter",
	}
}

// NewOrFilter 创建一个OR复合过滤器，任一子过滤器匹配即可
func NewOrFilter(filters ...MessageFilter) *CompositeFilter {
	return &CompositeFilter{
		filters:     filters,
		isAnd:       false,
		description: "OrFilter",
	}
}

// Match 判断消息是否匹配复合过滤规则
func (f *CompositeFilter) Match(message *Message) bool {
	if len(f.filters) == 0 {
		return true // 没有过滤器时默认匹配
	}

	if f.isAnd {
		// AND: 所有过滤器都必须匹配
		for _, filter := range f.filters {
			if !filter.Match(message) {
				return false
			}
		}
		return true
	} else {
		// OR: 任一过滤器匹配即可
		for _, filter := range f.filters {
			if filter.Match(message) {
				return true
			}
		}
		return false
	}
}

// GetDescription 获取过滤器的描述
func (f *CompositeFilter) GetDescription() string {
	if len(f.filters) == 0 {
		return f.description + "(empty)"
	}

	descriptions := make([]string, len(f.filters))
	for i, filter := range f.filters {
		descriptions[i] = filter.GetDescription()
	}

	op := " AND "
	if !f.isAnd {
		op = " OR "
	}

	return f.description + "(" + strings.Join(descriptions, op) + ")"
}

// TopicMatch 判断一个主题是否与主题过滤器匹配
// 支持MQTT的主题匹配规则：
// - "+" 匹配单个层级
// - "#" 匹配多个层级，但必须在最后
func TopicMatch(filter, topic string) bool {
	if filter == topic {
		return true
	}

	// 如果filter为"#"，匹配所有主题
	if filter == "#" {
		return true
	}

	// 将主题和过滤器按/分割
	topicLevels := strings.Split(topic, "/")
	filterLevels := strings.Split(filter, "/")

	// 处理特殊情况
	if len(filterLevels) > 0 && filterLevels[len(filterLevels)-1] == "#" {
		// 如果过滤器以#结尾，那么只需匹配前面的部分
		if len(filterLevels)-1 > len(topicLevels) {
			return false
		}
		for i := 0; i < len(filterLevels)-1; i++ {
			if filterLevels[i] != "+" && filterLevels[i] != topicLevels[i] {
				return false
			}
		}
		return true
	}

	// 如果层级数不同且过滤器不以#结尾，无法匹配
	if len(filterLevels) != len(topicLevels) {
		return false
	}

	// 逐级比较
	for i := 0; i < len(filterLevels); i++ {
		if filterLevels[i] != "+" && filterLevels[i] != topicLevels[i] {
			return false
		}
	}

	return true
}
