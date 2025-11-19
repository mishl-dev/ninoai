package bot

import (
	"fmt"
	"strings"
	"testing"

	"ninoai/pkg/cerebras"

	"github.com/bwmarrin/discordgo"
)

// Mock Cerebras Client
type mockCerebrasClient struct {
	ChatCompletionFunc func(messages []cerebras.Message) (string, error)
}

func (m *mockCerebrasClient) ChatCompletion(messages []cerebras.Message) (string, error) {
	if m.ChatCompletionFunc != nil {
		return m.ChatCompletionFunc(messages)
	}
	return "Default mock response", nil
}

// Mock Embedding Client
type mockEmbeddingClient struct {
	EmbedFunc func(text string) ([]float32, error)
}

func (m *mockEmbeddingClient) Embed(text string) ([]float32, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

// Mock Memory Store
type mockMemoryStore struct {
	AddFunc                 func(userId string, text string, vector []float32) error
	SearchFunc              func(userId string, queryVector []float32, limit int) ([]string, error)
	AddRecentMessageFunc    func(userId, message string) error
	GetRecentMessagesFunc   func(userId string) ([]string, error)
	ClearRecentMessagesFunc func(userId string) error
	DeleteUserDataFunc      func(userId string) error
}

func (m *mockMemoryStore) Add(userId string, text string, vector []float32) error {
	if m.AddFunc != nil {
		return m.AddFunc(userId, text, vector)
	}
	return nil
}

func (m *mockMemoryStore) Search(userId string, queryVector []float32, limit int) ([]string, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(userId, queryVector, limit)
	}
	return []string{"retrieved memory 1", "retrieved memory 2"}, nil
}

func (m *mockMemoryStore) AddRecentMessage(userId, message string) error {
	if m.AddRecentMessageFunc != nil {
		return m.AddRecentMessageFunc(userId, message)
	}
	return nil
}

func (m *mockMemoryStore) GetRecentMessages(userId string) ([]string, error) {
	if m.GetRecentMessagesFunc != nil {
		return m.GetRecentMessagesFunc(userId)
	}
	return []string{"recent message 1", "recent message 2"}, nil
}

func (m *mockMemoryStore) ClearRecentMessages(userId string) error {
	if m.ClearRecentMessagesFunc != nil {
		return m.ClearRecentMessagesFunc(userId)
	}
	return nil
}

func (m *mockMemoryStore) DeleteUserData(userId string) error {
	if m.DeleteUserDataFunc != nil {
		return m.DeleteUserDataFunc(userId)
	}
	return nil
}

// Mock Discord Session
type mockDiscordSession struct {
	ChannelMessageSendFunc func(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelTypingFunc      func(channelID string, options ...discordgo.RequestOption) error
	UserFunc               func(userID string) (*discordgo.User, error)
	ChannelFunc            func(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GuildEmojisFunc        func(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Emoji, error)
}

func (m *mockDiscordSession) ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ChannelMessageSendFunc != nil {
		return m.ChannelMessageSendFunc(channelID, content, options...)
	}
	return &discordgo.Message{}, nil
}

func (m *mockDiscordSession) ChannelTyping(channelID string, options ...discordgo.RequestOption) error {
	if m.ChannelTypingFunc != nil {
		return m.ChannelTypingFunc(channelID, options...)
	}
	return nil
}

func (m *mockDiscordSession) User(userID string) (*discordgo.User, error) {
	if m.UserFunc != nil {
		return m.UserFunc(userID)
	}
	return &discordgo.User{Username: "testuser"}, nil
}

func (m *mockDiscordSession) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if m.ChannelFunc != nil {
		return m.ChannelFunc(channelID, options...)
	}
	return &discordgo.Channel{Type: discordgo.ChannelTypeDM}, nil
}

func (m *mockDiscordSession) GuildEmojis(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Emoji, error) {
	if m.GuildEmojisFunc != nil {
		return m.GuildEmojisFunc(guildID, options...)
	}
	return []*discordgo.Emoji{}, nil
}

func TestMessageFlow(t *testing.T) {
	// Setup
	mockCerebras := &mockCerebrasClient{}
	mockEmbedding := &mockEmbeddingClient{}
	mockMemory := &mockMemoryStore{}
	mockSession := &mockDiscordSession{}

	handler := NewHandler(mockCerebras, mockEmbedding, mockMemory)
	handler.SetBotID("testbot")

	// Spies
	var searchCalled bool
	var getRecentMessagesCalled bool
	var addRecentMessageCalls int
	var addMemoryCalled bool
	var finalPrompt string

	mockMemory.SearchFunc = func(userId string, queryVector []float32, limit int) ([]string, error) {
		searchCalled = true
		return []string{"retrieved memory"}, nil
	}

	mockMemory.GetRecentMessagesFunc = func(userId string) ([]string, error) {
		getRecentMessagesCalled = true
		return []string{"rolling context"}, nil
	}

	mockCerebras.ChatCompletionFunc = func(messages []cerebras.Message) (string, error) {
		var promptBuilder strings.Builder
		isMemoryEvaluation := false
		userMessage := ""
		for _, msg := range messages {
			var role string
			switch msg.Role {
			case "system":
				role = "System"
			case "user":
				role = "User"
				userMessage = msg.Content
			default:
				role = "Unknown"
			}
			promptBuilder.WriteString(fmt.Sprintf("[%s]\n%s\n", role, msg.Content))

			if strings.Contains(msg.Content, "Analyze the following interaction") {
				isMemoryEvaluation = true
			}
		}
		if !isMemoryEvaluation {
			finalPrompt = promptBuilder.String()
		}

		if isMemoryEvaluation {
			if strings.Contains(userMessage, "Please remember") {
				return "YES", nil
			}
			return "NO", nil
		}

		if strings.Contains(userMessage, "remember") {
			return "[REMEMBER] This is a memorable response.", nil
		}

		return "This is a standard response.", nil
	}

	mockMemory.AddRecentMessageFunc = func(userId, message string) error {
		addRecentMessageCalls++
		return nil
	}

	mockMemory.AddFunc = func(userId string, text string, vector []float32) error {
		addMemoryCalled = true
		return nil
	}

	mockSession.ChannelTypingFunc = func(channelID string, options ...discordgo.RequestOption) error {
		return nil
	}

	mockSession.ChannelMessageSendFunc = func(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
		return &discordgo.Message{}, nil
	}

	// Test Case 1: Standard message
	t.Run("Standard flow", func(t *testing.T) {
		// Reset spies
		searchCalled = false
		getRecentMessagesCalled = false
		addRecentMessageCalls = 0
		addMemoryCalled = false
		finalPrompt = ""

		// Trigger
		handler.HandleMessage(mockSession, &discordgo.MessageCreate{
			Message: &discordgo.Message{
				Author:  &discordgo.User{ID: "user123", Username: "testuser"},
				Content: "Hello, this is a user message.",
				Mentions: []*discordgo.User{
					{ID: "testbot"},
				},
			},
		})
		handler.WaitForReady()

		// Assert
		if !searchCalled {
			t.Error("Expected Search to be called on memory store, but it wasn't")
		}
		if !getRecentMessagesCalled {
			t.Error("Expected GetRecentMessages to be called on memory store, but it wasn't")
		}
		if !strings.Contains(strings.ToLower(finalPrompt), "[system]") {
			t.Error("Final prompt does not contain the System Prompt")
		}
		if !strings.Contains(finalPrompt, "retrieved memory") {
			t.Error("Final prompt does not contain the Retrieved Memories")
		}
		if !strings.Contains(finalPrompt, "rolling context") {
			t.Error("Final prompt does not contain the Rolling Chat Context")
		}
		if !strings.Contains(finalPrompt, "Hello, this is a user message.") {
			t.Error("Final prompt does not contain the Current User Message")
		}
		if addRecentMessageCalls != 2 {
			t.Errorf("Expected AddRecentMessage to be called twice, but it was called %d times", addRecentMessageCalls)
		}
		if addMemoryCalled {
			t.Error("Expected Add (long-term memory) not to be called, but it was")
		}
	})

	// Test Case 2: Memorable message
	t.Run("Memorable flow", func(t *testing.T) {
		// Reset spies
		addMemoryCalled = false
		addRecentMessageCalls = 0
		searchCalled = false
		getRecentMessagesCalled = false
		finalPrompt = ""

		// Trigger
		handler.HandleMessage(mockSession, &discordgo.MessageCreate{
			Message: &discordgo.Message{
				Author:  &discordgo.User{ID: "user123", Username: "testuser"},
				Content: "Please remember this important fact.",
				Mentions: []*discordgo.User{
					{ID: "testbot"},
				},
			},
		})
		handler.WaitForReady()

		// Assert
		if !addMemoryCalled {
			t.Error("Expected Add (long-term memory) to be called, but it wasn't")
		}
	})
}
