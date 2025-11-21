package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ninoai/pkg/cerebras"
	"ninoai/pkg/memory"

	"github.com/bwmarrin/discordgo"
)

// Session interface abstracts discordgo.Session for testing
type Session interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendReply(channelID string, content string, reference *discordgo.MessageReference, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelTyping(channelID string, options ...discordgo.RequestOption) (err error)
	User(userID string) (*discordgo.User, error)
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GuildEmojis(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Emoji, error)
}

// DiscordSession adapts discordgo.Session to the Session interface
type DiscordSession struct {
	*discordgo.Session
}

func (s *DiscordSession) User(userID string) (*discordgo.User, error) {
	return s.Session.User(userID)
}

func (s *DiscordSession) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	return s.Session.Channel(channelID, options...)
}

func (s *DiscordSession) GuildEmojis(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Emoji, error) {
	return s.Session.GuildEmojis(guildID, options...)
}

type CerebrasClient interface {
	ChatCompletion(messages []cerebras.Message) (string, error)
}

type EmbeddingClient interface {
	Embed(text string) ([]float32, error)
}

type Classifier interface {
	Classify(text string, labels []string) (string, float64, error)
}

type Handler struct {
	cerebrasClient         CerebrasClient
	classifierClient       Classifier
	embeddingClient        EmbeddingClient
	memoryStore            memory.Store
	taskAgent              *TaskAgent
	botID                  string
	emojiCache             map[string][]string // guildID -> filtered emoji names
	emojiCacheMu           sync.RWMutex
	emojiCachePath         string // Path to emoji cache file
	wg                     sync.WaitGroup
	lastMessageTimes       map[string]time.Time
	lastMessageMu          sync.RWMutex
	messageProcessingDelay time.Duration
}

func NewHandler(c CerebrasClient, cl Classifier, e EmbeddingClient, m memory.Store, messageProcessingDelay float64) *Handler {
	h := &Handler{
		cerebrasClient:         c,
		classifierClient:       cl,
		embeddingClient:        e,
		memoryStore:            m,
		taskAgent:              NewTaskAgent(c, cl),
		emojiCache:             make(map[string][]string),
		emojiCachePath:         "storage/emoji_cache.json",
		lastMessageTimes:       make(map[string]time.Time),
		messageProcessingDelay: time.Duration(messageProcessingDelay * float64(time.Second)),
	}

	// Load emoji cache from disk
	h.loadEmojiCache()

	// Start a background goroutine to periodically clear inactive users' recent memory
	go h.clearInactiveUsers()

	return h
}

func (h *Handler) SetBotID(id string) {
	h.botID = id
}

func (h *Handler) addRecentMessage(userId, message string) {
	if err := h.memoryStore.AddRecentMessage(userId, message); err != nil {
		log.Printf("Error adding recent message: %v", err)
	}
}

func (h *Handler) getRecentMessages(userId string) []string {
	messages, err := h.memoryStore.GetRecentMessages(userId)
	if err != nil {
		log.Printf("Error getting recent messages: %v", err)
		return []string{}
	}
	return messages
}

func (h *Handler) ResetMemory(userId string) error {
	if err := h.memoryStore.ClearRecentMessages(userId); err != nil {
		log.Printf("Error clearing recent messages: %v", err)
	}
	// Also clear long-term memory for this user?
	// The user request said "ResetMemory" in the context of "Starting fresh".
	// If we want a full reset, we should call DeleteUserData.
	if err := h.memoryStore.DeleteUserData(userId); err != nil {
		log.Printf("Error deleting user data: %v", err)
	}
	return nil
}

// loadEmojiCache loads the emoji cache from disk
func (h *Handler) loadEmojiCache() {
	h.emojiCacheMu.Lock()
	defer h.emojiCacheMu.Unlock()

	data, err := os.ReadFile(h.emojiCachePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error loading emoji cache: %v", err)
		}
		return
	}

	var cache map[string][]string
	if err := json.Unmarshal(data, &cache); err != nil {
		log.Printf("Error unmarshaling emoji cache: %v", err)
		return
	}

	h.emojiCache = cache
	log.Printf("Loaded emoji cache with %d guilds", len(cache))
}

// saveEmojiCache saves the emoji cache to disk
func (h *Handler) saveEmojiCache() {
	h.emojiCacheMu.RLock()
	defer h.emojiCacheMu.RUnlock()

	data, err := json.MarshalIndent(h.emojiCache, "", "  ")
	if err != nil {
		log.Printf("Error marshaling emoji cache: %v", err)
		return
	}

	if err := os.MkdirAll(filepath.Dir(h.emojiCachePath), 0755); err != nil {
		log.Printf("Error creating cache directory: %v", err)
		return
	}

	if err := os.WriteFile(h.emojiCachePath, data, 0644); err != nil {
		log.Printf("Error saving emoji cache: %v", err)
	}
}

// filterRelevantEmojis uses LLM to filter emojis that are relevant to Nino's character
// Results are cached per guild to avoid redundant LLM calls
func (h *Handler) filterRelevantEmojis(guildID string, emojis []*discordgo.Emoji) []string {
	if len(emojis) == 0 {
		return []string{}
	}

	// Check cache first
	h.emojiCacheMu.RLock()
	if cached, ok := h.emojiCache[guildID]; ok {
		h.emojiCacheMu.RUnlock()
		return cached
	}
	h.emojiCacheMu.RUnlock()

	// Build emoji list for filtering
	var emojiNames []string
	for _, emoji := range emojis {
		emojiNames = append(emojiNames, emoji.Name)
	}

	// If there are too many emojis, just take the first 50 to avoid token limits
	if len(emojiNames) > 50 {
		emojiNames = emojiNames[:50]
	}

	filterPrompt := fmt.Sprintf(`You are filtering custom Discord emojis for Nino Nakano, a tsundere character who loves cooking, fashion, and romance.

Emoji names to filter: %s

Select ONLY emojis that are relevant to Nino's character and interests:
- Cooking, food, tea, restaurants
- Fashion, style, beauty
- Romance, love, hearts
- Emotions (happy, sad, angry, embarrassed, etc.)
- General reactions (thumbs up, wave, etc.)

EXCLUDE emojis related to:
- Gaming, esports, tech
- Sports, fitness
- Memes or internet culture (unless very general)
- Other anime characters (unless from Quintessential Quintuplets)
- Random/nonsensical names

Return ONLY the emoji names that should be kept, separated by commas. If none are relevant, return "NONE".`, strings.Join(emojiNames, ", "))

	messages := []cerebras.Message{
		{Role: "system", Content: "You are an emoji filter for a character AI."},
		{Role: "user", Content: filterPrompt},
	}

	resp, err := h.cerebrasClient.ChatCompletion(messages)
	if err != nil {
		log.Printf("Error filtering emojis: %v", err)
		// If filtering fails, return first 10 emojis as fallback
		if len(emojiNames) > 10 {
			return emojiNames[:10]
		}
		return emojiNames
	}

	// Parse response
	var result []string
	if strings.TrimSpace(resp) == "NONE" {
		result = []string{}
	} else {
		// Split by comma and clean up
		filtered := strings.Split(resp, ",")
		for _, name := range filtered {
			cleaned := strings.TrimSpace(name)
			if cleaned != "" {
				result = append(result, cleaned)
			}
		}
	}

	// Cache the result
	h.emojiCacheMu.Lock()
	h.emojiCache[guildID] = result
	h.emojiCacheMu.Unlock()

	// Save cache to disk (async to avoid blocking)
	go h.saveEmojiCache()

	return result
}

func (h *Handler) MessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	h.HandleMessage(&DiscordSession{s}, m)
}

func (h *Handler) HandleMessage(s Session, m *discordgo.MessageCreate) {
	// Ignore own messages
	if m.Author.ID == h.botID {
		return
	}

	// Ignore long messages
	if len(m.Content) > 280 {
		s.ChannelMessageSendReply(m.ChannelID, "...", m.Reference())
		return
	}

	// Update last message time for the user to track activity
	h.updateLastMessageTime(m.Author.ID)

	// Get channel info to check if it's a DM
	channel, err := s.Channel(m.ChannelID)
	isDM := err == nil && channel.Type == discordgo.ChannelTypeDM

	// Check if mentioned
	isMentioned := false
	for _, user := range m.Mentions {
		if user.ID == h.botID {
			isMentioned = true
			break
		}
	}

	// If the message is a reply, ignore it unless it's a reply to the bot
	if m.MessageReference != nil {
		// To get the message being replied to, you might need to fetch it
		// For now, let's assume if it's a reply and we are not mentioned, we ignore it.
		// A more robust solution would be to check if the replied-to message was from the bot.
		if !isMentioned {
			return
		}
	}

	// Decision Logic: Should I reply?
	// Always reply in DMs, otherwise use decision logic
	shouldReply := isMentioned || isDM

	// Get recent context (Rolling Chat Context)
	recentMsgs := h.getRecentMessages(m.Author.ID)

	if !shouldReply {
		// Use Classifier to decide if Nino should respond based on her personality
		labels := []string{
			"message directly addressing Nino or Nakano",
			"discussion about cooking or food",
			"discussion about fashion or appearance",
			"discussion about romance or relationships",
			"someone being pathetic",
			"someone not taking care of themselves (health, sleep, eating)",
			"casual conversation or blank message",
		}

		label, score, err := h.classifierClient.Classify(m.Content, labels)
		if err != nil {
			log.Printf("Error classifying message: %v", err)
		} else {
			log.Printf("Reply Decision: '%s' (score: %.2f)", label, score)
			// Reply if it matches her personality triggers (not casual conversation)
			if label != "casual conversation unrelated to Nino's interests" && score > 0.6 {
				shouldReply = true
			}
		}
	}

	if !shouldReply {
		// Even if not replying, we might want to add to recent context?
		// For now, let's only add if we reply or are involved.
		// Actually, if we don't reply, we should probably NOT add it to context
		// unless we want to track "overheard" conversations.
		// Let's stick to adding only when we reply for now to keep context clean.
		return
	}

	// Prepare display name
	displayName := m.Author.Username
	if m.Author.GlobalName != "" {
		displayName = m.Author.GlobalName
	}

	s.ChannelTyping(m.ChannelID)

	// Check if this is a long task request that should be refused
	isTask, refusal := h.taskAgent.CheckTask(m.Content)
	if isTask {
		h.sendSplitMessage(s, m.ChannelID, refusal, m.Reference())

		// Record the refusal in recent memory
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			h.addRecentMessage(m.Author.ID, fmt.Sprintf("%s: %s", displayName, m.Content))
			h.addRecentMessage(m.Author.ID, fmt.Sprintf("Nino: %s", refusal))
		}()
		return
	}

	// 1. Generate Embedding for current message
	// We use the user's message as the query for retrieval
	emb, err := h.embeddingClient.Embed(m.Content)
	if err != nil {
		log.Printf("Error generating embedding: %v", err)
	}

	// 2. Search Memory (RAG)
	var retrievedMemories string
	if emb != nil {
		matches, err := h.memoryStore.Search(m.Author.ID, emb, 5) // Top 5 relevant memories
		if err != nil {
			log.Printf("Error searching memory: %v", err)
		} else if len(matches) > 0 {
			retrievedMemories = "Relevant past memories:\n- " + strings.Join(matches, "\n- ")
		}
	}

	// 3. Prepare Context (Rolling Window)
	// We already fetched recentMsgs above.
	var rollingContext string
	if len(recentMsgs) > 0 {
		rollingContext = "Recent conversation:\n" + strings.Join(recentMsgs, "\n")
	}

	// 4. Prepare Emojis
	var emojiText string
	if channel != nil && channel.GuildID != "" {
		emojis, err := s.GuildEmojis(channel.GuildID)
		if err == nil && len(emojis) > 0 {
			relevantNames := h.filterRelevantEmojis(channel.GuildID, emojis)

			if len(relevantNames) > 0 {
				nameToEmoji := make(map[string]*discordgo.Emoji)
				for _, emoji := range emojis {
					nameToEmoji[emoji.Name] = emoji
				}

				var emojiList []string
				for _, name := range relevantNames {
					if emoji, ok := nameToEmoji[name]; ok {
						emojiList = append(emojiList, fmt.Sprintf("<:%s:%s>", emoji.Name, emoji.ID))
					}
				}

				if len(emojiList) > 0 {
					emojiText = "Available custom emojis:\n" + strings.Join(emojiList, ", ")
				}
			}
		}
	}

	// 5. Construct Prompt
	// [System Prompt]
	// [Retrieved Memories]
	// [Rolling Chat Context]
	// [Current User Message] (handled by appending as user message)

	systemPrompt := fmt.Sprintf(SystemPrompt, displayName)
	messages := []cerebras.Message{
		{Role: "system", Content: systemPrompt},
	}
	log.Printf("Retrieved memories: %s", retrievedMemories)
	if retrievedMemories != "" {
		messages = append(messages, cerebras.Message{Role: "system", Content: retrievedMemories})
	}
	log.Printf("Rolling context: %s", rollingContext)
	if rollingContext != "" {
		messages = append(messages, cerebras.Message{Role: "system", Content: rollingContext})
	}
	if emojiText != "" {
		messages = append(messages, cerebras.Message{Role: "system", Content: emojiText})
	}

	messages = append(messages, cerebras.Message{Role: "user", Content: m.Content})

	// 6. Generate Reply
	reply, err := h.cerebrasClient.ChatCompletion(messages)
	if err != nil {
		log.Printf("Error getting completion: %v", err)
		h.sendSplitMessage(s, m.ChannelID, "(I'm having a headache... try again later.)", m.Reference())
		return
	}

	// Check for memory tag to clean up the user-facing message
	displayReply := reply
	if idx := strings.Index(reply, "[MEMORY:"); idx != -1 {
		displayReply = strings.TrimSpace(reply[:idx])
	}

	h.sendSplitMessage(s, m.ChannelID, displayReply, m.Reference())

	// 7. Async Updates
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		// Check for memory tag in the reply
		finalReply := reply
		var memoryFact string

		if idx := strings.Index(reply, "[MEMORY:"); idx != -1 {
			// Extract memory
			memoryContent := reply[idx:]
			if endIdx := strings.Index(memoryContent, "]"); endIdx != -1 {
				memoryFact = strings.TrimSpace(memoryContent[8:endIdx])
				// Remove tag from the reply that goes to context
				finalReply = strings.TrimSpace(reply[:idx])
			}
		}

		// Add to Rolling Context
		h.addRecentMessage(m.Author.ID, fmt.Sprintf("%s: %s", displayName, m.Content))
		h.addRecentMessage(m.Author.ID, fmt.Sprintf("Nino: %s", finalReply))

		// Store extracted memory if present
		if memoryFact != "" {
			// Validate memory importance
			if !h.isMemoryWorthStoring(memoryFact) {
				log.Printf("Skipping trivial memory: %s", memoryFact)
				return
			}

			log.Printf("Detected memory update: %s", memoryFact)
			factEmb, err := h.embeddingClient.Embed(memoryFact)
			if err == nil {
				log.Printf("Storing new memory for user %s: %s", m.Author.ID, memoryFact)
				if err := h.memoryStore.Add(m.Author.ID, memoryFact, factEmb); err != nil {
					// Check if this is a duplicate error
					if strings.Contains(err.Error(), "duplicate memory") {
						log.Printf("Skipping duplicate memory: %v", err)
					} else {
						log.Printf("Error storing memory: %v", err)
					}
				}
			}
		}
	}()
}

func (h *Handler) isMemoryWorthStoring(fact string) bool {
	lower := strings.ToLower(fact)

	// Filter out meta-commentary
	if strings.Contains(lower, "no info") || strings.Contains(lower, "no new info") {
		return false
	}

	// Filter out trivial behavior observations
	if strings.Contains(lower, "emoticon") || strings.Contains(lower, "emoji") {
		return false
	}

	// Filter out very short memories
	if len(fact) < 5 {
		return false
	}

	return true
}

func (h *Handler) sendSplitMessage(s Session, channelID, content string, reference *discordgo.MessageReference) {
	// Replace \n\n with a special separator for multi-part messages
	content = strings.ReplaceAll(content, "\n\n", "---SPLIT---")
	parts := strings.Split(content, "---SPLIT---")

	isFirstPart := true
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var err error
		if reference == nil {
			// If there's no reference, send a normal message
			_, err = s.ChannelMessageSend(channelID, part)
		} else {
			if isFirstPart {
				// The first part of a reply pings the user by default
				_, err = s.ChannelMessageSendReply(channelID, part, reference)
				isFirstPart = false
			} else {
				// Subsequent parts are sent as replies without pinging the user
				_, err = s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
					Content:   part,
					Reference: reference,
					AllowedMentions: &discordgo.MessageAllowedMentions{
						RepliedUser: false, // This prevents pinging on subsequent parts
					},
				})
			}
		}

		if err != nil {
			log.Printf("Error sending message part: %v", err)
		}

		// Add a short delay between messages for a more natural feel
		time.Sleep(h.messageProcessingDelay)
	}
}

func (h *Handler) updateLastMessageTime(userID string) {
	h.lastMessageMu.Lock()
	defer h.lastMessageMu.Unlock()
	h.lastMessageTimes[userID] = time.Now()
}

func (h *Handler) clearInactiveUsers() {
	// Check for inactive users every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.lastMessageMu.Lock()
		for userID, lastTime := range h.lastMessageTimes {
			// If user has been inactive for 30 minutes, clear their recent memory
			if time.Since(lastTime) > 30*time.Minute {
				log.Printf("User %s has been inactive for 30 minutes, clearing recent memory", userID)
				if err := h.memoryStore.ClearRecentMessages(userID); err != nil {
					log.Printf("Error clearing recent messages for inactive user %s: %v", userID, err)
				}
				// Remove from tracking map
				delete(h.lastMessageTimes, userID)
			}
		}
		h.lastMessageMu.Unlock()
	}
}

func (h *Handler) WaitForReady() {
	h.wg.Wait()
}
