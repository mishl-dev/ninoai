package bot

import (
	"fmt"
	"log"
	"strings"

	"ninoai/pkg/cerebras"
)

type MemoryAgent struct {
	client CerebrasClient
}

func NewMemoryAgent(client CerebrasClient) *MemoryAgent {
	return &MemoryAgent{
		client: client,
	}
}

func (ma *MemoryAgent) EvaluateMemory(userMsg, botReply string) (bool, string) {
	if strings.Contains(botReply, "[REMEMBER]") {
		return true, userMsg
	}

	prompt := fmt.Sprintf(`Analyze the following interaction between a user and Nino (AI).
	
User: "%s"

Does this interaction contain important facts, preferences, specific events, or relationship developments that should be remembered long-term?

If YES, extract the core fact or information to be stored in a concise, standalone sentence (e.g., "User's favorite food is ramen", "User is a software engineer").
If NO, reply with exactly "[NO]". and nothing else.

Examples:
- User: "My name is John." -> "User's name is John."
- User: "I love spicy food." -> "User loves spicy food."
- User: "Hi" -> "[NO]"
- User: "What is the weather?" -> "[NO]"
- User: "I like ramen." -> "User's favorite food is ramen."
- User: "What is my favorite food?" -> "[NO]"
`, userMsg)

	messages := []cerebras.Message{
		{Role: "system", Content: "You are a memory manager for an AI. Extract key facts."},
		{Role: "user", Content: prompt},
	}

	resp, err := ma.client.ChatCompletion(messages)
	if err != nil {
		log.Printf("Error evaluating memory: %v", err)
		return false, ""
	}

	cleaned := strings.TrimSpace(resp)
	cleanedUpper := strings.ToUpper(cleaned)
	
	// Check if response is exactly [NO] or contains [NO]
	if cleanedUpper == "[NO]" || strings.Contains(cleanedUpper, "[NO]") {
		return false, ""
	}
	
	// Remove quotes from memory content before saving
	finalMemory := strings.Trim(cleaned, "\"")

	return true, finalMemory
}