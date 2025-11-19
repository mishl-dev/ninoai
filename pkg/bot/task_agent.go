package bot

import (
	"fmt"
	"log"
	"strings"

	"ninoai/pkg/cerebras"
)

type TaskAgent struct {
	client CerebrasClient
}

func NewTaskAgent(client CerebrasClient) *TaskAgent {
	return &TaskAgent{
		client: client,
	}
}

// CheckTask analyzes if the message is a long writing task.
// If it is, it returns true and a refusal message in character.
// If not, it returns false and an empty string.
func (ta *TaskAgent) CheckTask(userMsg string) (bool, string) {
	prompt := fmt.Sprintf(`Analyze the following user message to Nino Nakano (a Discord bot with a Tsundere personality).

User Message: "%s"

Is the user asking for a LONG or COMPLEX writing task?
Examples of such tasks:
- "Write a 4000-word essay about..."
- "Write a story about..." (if it implies length)
- "Generate code for..."
- "Write an article about..."
- "Summarize this entire book..."

You are NOT to fulfill the request. You are to DETECT if it is a request for a long/complex output that an AI tool would do, but a "girl on Discord" would refuse.

If YES (it is a long task):
Reply as Nino Nakano. Refuse the request immediately.
- Be mocking and arrogant.
- Tell them you are not their secretary or AI assistant.
- Tell them to do their own homework.
- Keep it short (1-2 sentences).
- Do NOT start with "Nino:" or quotes. Just the message.

If NO (it is a normal chat message, question, or short request):
Reply with exactly "[NO]".

Response:`, userMsg)

	messages := []cerebras.Message{
		{Role: "system", Content: "You are a task filter agent. You detect long writing tasks and generate refusals."},
		{Role: "user", Content: prompt},
	}

	resp, err := ta.client.ChatCompletion(messages)
	if err != nil {
		log.Printf("Error checking task: %v", err)
		return false, ""
	}

	cleaned := strings.TrimSpace(resp)

	if strings.Contains(strings.ToUpper(cleaned), "[NO]") {
		return false, ""
	}

	return true, cleaned
}
