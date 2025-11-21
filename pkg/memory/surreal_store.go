package memory

import (
	"fmt"
	"log"
	"ninoai/pkg/surreal"
	"time"
)

type SurrealStore struct {
	client *surreal.Client
}

type SurrealMemoryItem struct {
	ID        string    `json:"id,omitempty"`
	UserID    string    `json:"user_id"`
	Text      string    `json:"text"`
	Embedding []float32 `json:"vector"`
	Timestamp int64     `json:"timestamp"`
}

type RecentMessageItem struct {
	ID        string `json:"id,omitempty"`
	UserID    string `json:"user_id"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

func NewSurrealStore(client *surreal.Client) *SurrealStore {
	store := &SurrealStore{
		client: client,
	}
	if err := store.Init(); err != nil {
		// Log error but don't fail startup, as DB might be reachable later or schema exists
		// In production, you might want to handle this more strictly
		fmt.Printf("Warning: Failed to initialize SurrealDB schema: %v\n", err)
	}
	return store
}

func (s *SurrealStore) Init() error {
	// Define schema for memories
	// We use a transaction-like block or just sequential queries
	query := `
		DEFINE TABLE IF NOT EXISTS memories SCHEMAFULL;
		DEFINE FIELD IF NOT EXISTS user_id ON memories TYPE string;
		DEFINE FIELD IF NOT EXISTS text ON memories TYPE string;
		DEFINE FIELD IF NOT EXISTS timestamp ON memories TYPE int;
		-- We define the vector field with 768 dimensions
		DEFINE FIELD IF NOT EXISTS vector ON memories TYPE array<float> ASSERT array::len($value) == 768;
		DEFINE INDEX IF NOT EXISTS vector_idx ON memories FIELDS vector MTREE DIMENSION 768 DIST COSINE;

		DEFINE TABLE IF NOT EXISTS recent_messages SCHEMAFULL;
		DEFINE FIELD IF NOT EXISTS user_id ON recent_messages TYPE string;
		DEFINE FIELD IF NOT EXISTS text ON recent_messages TYPE string;
		DEFINE FIELD IF NOT EXISTS timestamp ON recent_messages TYPE int;
	`
	_, err := s.client.Query(query, map[string]interface{}{})
	return err
}

func (s *SurrealStore) Add(userId string, text string, vector []float32) error {
	item := SurrealMemoryItem{
		UserID:    userId,
		Text:      text,
		Embedding: vector,
		Timestamp: time.Now().Unix(),
	}

	_, err := s.client.Create("memories", item)
	return err
}

func (s *SurrealStore) Search(userId string, queryVector []float32, limit int) ([]string, error) {
	log.Printf("[DEBUG] Search called: userId=%s, vectorLen=%d, limit=%d", userId, len(queryVector), limit)

	// Use the client's VectorSearch method to avoid raw queries in the store
	rows, err := s.client.VectorSearch("memories", "vector", queryVector, limit, map[string]interface{}{
		"user_id": userId,
	})
	if err != nil {
		log.Printf("[DEBUG] VectorSearch error: %v", err)
		return nil, err
	}

	log.Printf("[DEBUG] VectorSearch returned %d rows", len(rows))

	const similarityThreshold = 0.6 // Only include memories with good similarity
	var texts []string

	for _, row := range rows {
		if rowMap, ok := row.(map[string]interface{}); ok {
			if text, ok := rowMap["text"].(string); ok {
				similarity := rowMap["similarity"]

				// Check if similarity meets threshold
				var simScore float64
				switch v := similarity.(type) {
				case float64:
					simScore = v
				case float32:
					simScore = float64(v)
				default:
					log.Printf("Unknown similarity type: %T", similarity)
					continue
				}

				if simScore >= similarityThreshold {
					log.Printf("Memory match: '%s' (similarity: %.4f)", text, simScore)
					texts = append(texts, text)
				} else {
					log.Printf("Skipping low-similarity memory: '%s' (similarity: %.4f)", text, simScore)
				}
			}
		}
	}

	return texts, nil
}

// Recent messages cache

func (s *SurrealStore) AddRecentMessage(userId, message string) error {
	item := RecentMessageItem{
		UserID:    userId,
		Text:      message,
		Timestamp: time.Now().Unix(),
	}

	_, err := s.client.Create("recent_messages", item)
	if err != nil {
		return err
	}

	// Cleanup old messages (keep last 15)
	// Changed to SELECT id, timestamp (not SELECT VALUE) since we need to order by timestamp
	query := `
		DELETE recent_messages
		WHERE user_id = $user_id
		AND id NOT IN (
			SELECT id, timestamp FROM recent_messages
			WHERE user_id = $user_id
			ORDER BY timestamp DESC
			LIMIT 15
		).id;
	`
	_, err = s.client.Query(query, map[string]interface{}{"user_id": userId})
	return err
}

func (s *SurrealStore) GetRecentMessages(userId string) ([]string, error) {
	// Include 'timestamp' in SELECT since we're ordering by it
	query := `
		SELECT text, timestamp FROM recent_messages
		WHERE user_id = $user_id
		ORDER BY timestamp ASC;
	`

	result, err := s.client.Query(query, map[string]interface{}{"user_id": userId})
	if err != nil {
		return nil, err
	}

	resSlice, ok := result.([]interface{})
	if !ok || len(resSlice) == 0 {
		return []string{}, nil
	}

	queryRes := resSlice[0]
	var rows []interface{}
	if resMap, ok := queryRes.(map[string]interface{}); ok {
		if val, ok := resMap["result"]; ok {
			if r, ok := val.([]interface{}); ok {
				rows = r
			}
		}
	} else if r, ok := queryRes.([]interface{}); ok {
		rows = r
	}

	var messages []string
	for _, row := range rows {
		if rowMap, ok := row.(map[string]interface{}); ok {
			if text, ok := rowMap["text"].(string); ok {
				messages = append(messages, text)
			}
		}
	}

	return messages, nil
}

func (s *SurrealStore) ClearRecentMessages(userId string) error {
	query := `DELETE recent_messages WHERE user_id = $user_id;`
	_, err := s.client.Query(query, map[string]interface{}{"user_id": userId})
	return err
}

func (s *SurrealStore) DeleteUserData(userId string) error {
	query := `
		DELETE memories WHERE user_id = $user_id;
		DELETE recent_messages WHERE user_id = $user_id;
	`
	_, err := s.client.Query(query, map[string]interface{}{"user_id": userId})
	return err
}
