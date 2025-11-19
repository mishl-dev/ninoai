package bot

import (
	"fmt"
	"log"
	"strings"

	"ninoai/pkg/cerebras"
)

type MemoryAgent struct {
	client *cerebras.Client
}

func NewMemoryAgent(client *cerebras.Client) *MemoryAgent {
	return &MemoryAgent{
		client: client,
	}
}

func (ma *MemoryAgent) EvaluateMemory(userMsg, botReply string) bool {
	prompt := fmt.Sprintf(`Analyze the following interaction between a user and Nino (AI).
	
User: "%s"
Nino: "%s"

Does this interaction contain important facts, preferences, specific events, or relationship developments that should be remembered long-term?
Examples of YES:
- User mentions their name, age, location, or job.
- User states a strong preference (likes/dislikes).
- A significant event happens (confession, argument, agreement).
- Nino learns something new about the user.

Examples of NO:
- Small talk (greetings, "how are you").
- Random jokes or memes without context.
- Short, meaningless exchanges.
- Repetitive information.

Reply with exactly "YES" or "NO".`, userMsg, botReply)

	messages := []cerebras.Message{
		{Role: "system", Content: "You are a memory manager for an AI."},
		{Role: "user", Content: prompt},
	}

	resp, err := ma.client.ChatCompletion(messages)
	if err != nil {
		log.Printf("Error evaluating memory: %v", err)
		return false
	}

	cleaned := strings.TrimSpace(strings.ToUpper(resp))
	return strings.Contains(cleaned, "YES")
}
