package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// InputMessage 输入消息格式（与Anthropic API兼容）
// 独立于api包定义，避免memory包对api包的依赖
type InputMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// ParsedRequest 解析后的请求结构
type ParsedRequest struct {
	FixedPrompt    string   // 固定提示词（从system字段提取）
	PromptHash     string   // 固定提示词指纹
	MemoryEntries  []string // 记忆内容（从assistant历史消息提取）
	CurrentRequest string   // 当前需求（最后一条user消息）
}

// MessageParser 消息解析器
type MessageParser struct{}

// NewMessageParser 创建消息解析器实例
func NewMessageParser() *MessageParser {
	return &MessageParser{}
}

// ParseMessages 解析消息列表，提取固定提示词、记忆内容和当前请求
// 参数 messages 为符合Anthropic API格式的消息数组
// 参数 system 为system字段，支持string、[]interface{}等多种格式
func (p *MessageParser) ParseMessages(messages []InputMessage, system interface{}) *ParsedRequest {
	result := &ParsedRequest{}

	// 1. 提取固定提示词
	result.FixedPrompt = p.extractSystemPrompt(system)

	// 2. 计算固定提示词指纹
	result.PromptHash = p.computeFingerprint(result.FixedPrompt)

	// 3. 遍历消息，提取assistant记忆和user当前请求
	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			// 提取assistant消息作为记忆内容
			text := p.extractTextFromContent(msg.Content)
			if text != "" {
				result.MemoryEntries = append(result.MemoryEntries, text)
			}
		case "user":
			// 最后一条user消息作为当前请求（后续会覆盖）
			text := p.extractTextFromContent(msg.Content)
			if text != "" {
				result.CurrentRequest = text
			}
		}
	}

	return result
}

// extractSystemPrompt 从system字段提取系统提示词
// 支持多种格式：
//   - nil: 返回空字符串
//   - string: 直接返回
//   - []interface{}: 遍历提取type="text"的内容
//   - 其他: 使用fmt.Sprintf转换为字符串
func (p *MessageParser) extractSystemPrompt(system interface{}) string {
	if system == nil {
		return ""
	}

	switch s := system.(type) {
	case string:
		return s
	case []interface{}:
		var texts []string
		for _, item := range s {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["type"] == "text" {
					if text, ok := itemMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		return strings.Join(texts, "\n")
	default:
		return fmt.Sprintf("%v", system)
	}
}

// extractTextFromContent 从content字段提取文本内容
// 支持多种格式：
//   - string: 直接返回
//   - []interface{}: 遍历提取type="text"的text字段，以及type="tool_result"的content
//   - 其他: 使用fmt.Sprintf转换为字符串
func (p *MessageParser) extractTextFromContent(content interface{}) string {
	if content == nil {
		return ""
	}

	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var texts []string
		for _, item := range c {
			if itemMap, ok := item.(map[string]interface{}); ok {
				itemType, _ := itemMap["type"].(string)

				switch itemType {
				case "text":
					// 提取text类型的内容
					if text, ok := itemMap["text"].(string); ok {
						texts = append(texts, text)
					}
				case "tool_result":
					// 提取tool_result类型的content
					if toolContent, ok := itemMap["content"]; ok {
						switch tc := toolContent.(type) {
						case string:
							texts = append(texts, tc)
						case []interface{}:
							// tool_result的content可能是嵌套数组
							for _, subItem := range tc {
								if subMap, ok := subItem.(map[string]interface{}); ok {
									if subMap["type"] == "text" {
										if text, ok := subMap["text"].(string); ok {
											texts = append(texts, text)
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return strings.Join(texts, "\n")
	default:
		return fmt.Sprintf("%v", content)
	}
}

// computeFingerprint 计算内容的指纹（SHA256哈希前8位）
// 对内容先做TrimSpace和统一换行符处理，确保相同语义内容产生相同指纹
func (p *MessageParser) computeFingerprint(content string) string {
	// 预处理：去除首尾空白，统一换行符为\n
	normalized := strings.TrimSpace(content)
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// 空内容返回空指纹
	if normalized == "" {
		return ""
	}

	// 计算SHA256哈希，取前8位作为指纹
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:8]
}
