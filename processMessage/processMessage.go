package processMessage

import (
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"hash/crc32"
	"strings"
)

type Message struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type ProcessData interface {
	GenerateMessageID() (string, error)
	GenerateMessageUniqueTags(messages []Message) (string, error)
	GenerateMessage(messages []Message) string
	GinGenerateMessage()
}

// validateMessages 检查消息顺序（防用户的，唉）
func validateMessages(messages []Message) bool {
	if len(messages) == 0 {
		return false
	}

	// 检查System是否出现在非第一个位置
	for i := 1; i < len(messages); i++ {
		if messages[i].Role == "system" {
			return false
		}
	}

	hasSystem := messages[0].Role == "system"

	// 处理System存在的情况
	startIndex := 0
	expectedRole := "user"
	if hasSystem {
		// 如果只有System一个消息，无效（因为最后需要是User）
		if len(messages) == 1 {
			return false
		}
		// System后的第一个必须是User
		if messages[1].Role != "user" {
			return false
		}
		expectedRole = "assistant"
		startIndex = 2
	} else {
		// 没有System时，第一个必须是User
		if messages[0].Role != "user" {
			return false
		}
		expectedRole = "assistant"
		startIndex = 1
	}

	// 检查后续角色是否交替
	for i := startIndex; i < len(messages); i++ {
		if messages[i].Role != expectedRole {
			return false
		}
		// 切换期望的角色
		if expectedRole == "user" {
			expectedRole = "assistant"
		} else {
			expectedRole = "user"
		}
	}

	// 确保最后一个角色是User
	return messages[len(messages)-1].Role == "user"
}

// GenerateMessageID 生成一个随机uuid作为消息ID，可以自定义
func GenerateMessageID() (string, error) {
	return uuid.NewV4().String(), nil
}

// GenerateMessageUniqueTags 默认使用 CRC32 算法计算唯一标签
func GenerateMessageUniqueTags(previousConversations, userMessage []Message) (string, error) {
	var checksums strings.Builder
	// 生成CRC20校验和
	generateCRC20 := func(Message []Message) error {
		// 序列化为紧凑 JSON
		data, err := json.Marshal(Message)
		if err != nil {
			return fmt.Errorf("json marshal failed: %v", err)
		}
		checksums.WriteString(fmt.Sprintf("%08x", crc32.ChecksumIEEE(data)))
		checksums.WriteString("-")
		return nil
	}
	if err := generateCRC20(previousConversations); err != nil {
		return "", err
	}
	checksums.WriteString("-")
	if err := generateCRC20(userMessage); err != nil {
		return "", err
	}
	return checksums.String(), nil
}

// GenerateSplitMessageUniqueTags 将消息拆分并计算唯一标签
func GenerateSplitMessageUniqueTags(messages []Message) (string, error) {
	if validateMessages(messages) {
		return "", fmt.Errorf("messages format is invalid")
	}
	var previousConversations []Message
	if len(messages) <= 2 {
		previousConversations = messages
	} else {
		// 当消息数 >2 时，Privious 取最后两条之前的消息，即assistant和user
		previousConversations = messages[:len(messages)-2]
	}
	var checksums strings.Builder
	// 生成CRC20校验和
	generateCRC20 := func(Message []Message) error {
		// 序列化为紧凑 JSON
		data, err := json.Marshal(Message)
		if err != nil {
			return fmt.Errorf("json marshal failed: %v", err)
		}
		checksums.WriteString(fmt.Sprintf("%08x", crc32.ChecksumIEEE(data)))
		return nil
	}
	if err := generateCRC20(previousConversations); err != nil {
		return "", err
	}
	return checksums.String(), nil
}

// GenerateMessage 组合为msg.Role: msg.Content的格式
func GenerateMessage(messages []Message) (string, error) {
	if validateMessages(messages) {
		return "", fmt.Errorf("messages has consecutive roles or last role is not User")
	}
	lines := make([]string, len(messages))
	for i, msg := range messages {
		lines[i] = fmt.Sprintf("%s: %s", msg.Role, msg.Content)
	}
	return strings.Join(lines, "\n"), nil
}
