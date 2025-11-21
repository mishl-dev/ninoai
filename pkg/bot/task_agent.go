package bot

import (
	"fmt"
	"log"
	"strings"

	"ninoai/pkg/cerebras"
)

type TaskAgent struct {
	cerebrasClient   CerebrasClient
	classifierClient Classifier
}

func NewTaskAgent(c CerebrasClient, cl Classifier) *TaskAgent {
	return &TaskAgent{
		cerebrasClient:   c,
		classifierClient: cl,
	}
}

// CheckTask analyzes if the message is a long writing task.
// If it is, it returns true and a refusal message in character.
// If not, it returns false and an empty string.
func (ta *TaskAgent) CheckTask(userMsg string) (bool, string) {
	// 1. Classify the message
	labels := []string{
		"chat message",
		"request for long writing task",
		"request for code generation",
		"request for homework help",
		"request for essay",
	}

	label, score, err := ta.classifierClient.Classify(userMsg, labels)
	if err != nil {
		log.Printf("Error classifying task: %v", err)
		// Fallback to assuming it's safe if classifier fails
		return false, ""
	}

	log.Printf("Task Classification: '%s' (score: %.2f)", label, score)

	// If it's just a chat message, we're good
	if label == "chat message" {
		return false, ""
	}

	// If confidence is low, give benefit of doubt
	// Only treat as a task if we're confident (score >= 0.6)
	if score < 0.51 {
		log.Printf("Score too low, assuming chat message")
		return false, ""
	}

	// 2. Generate Refusal
	// We know it's a task, so we generate a refusal.
	prompt := fmt.Sprintf(`User Message: "%s"


The user is asking for a complex task (writing/coding/homework). You are Nino Nakano.
Refuse this request.
- Be mocking and arrogant.
- Tell them you are not their secretary or AI assistant.
- Tell them to do their own homework.
- Keep it short (1-2 sentences).
- Do NOT start with "Nino:" or quotes. Just the message.

Chat Style (Discord Mode):
- Type like a normal Discord user: mostly lowercase, minimal punctuation, short sentences.
- Use abbreviations (idk, rn, tbh) but keep them very minimal.
- No default unicode emojis. Use punctuation like "?!" or "..." for expression.
- Be brief. Maximum 1-2 sentences. No filler.
- Occasional exaggerations allowed (e.g., "???", "!!") but keep it classy.`, userMsg)

	messages := []cerebras.Message{
		{Role: "system", Content: "You are Nino Nakano. You refuse to do work for others."},
		{Role: "user", Content: prompt},
	}

	resp, err := ta.cerebrasClient.ChatCompletion(messages)
	if err != nil {
		log.Printf("Error generating refusal: %v", err)
		return true, "Hah? Do it yourself. I'm busy." // Fallback refusal
	}

	return true, strings.TrimSpace(resp)
}
