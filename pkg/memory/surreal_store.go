package memory

import (
	"fmt"
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
	Vector    []float32 `json:"vector"`
	Timestamp int64     `json:"timestamp"`
}

type RecentMessageItem struct {
	ID        string `json:"id,omitempty"`
	UserID    string `json:"user_id"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

func NewSurrealStore(client *surreal.Client) *SurrealStore {
	return &SurrealStore{
		client: client,
	}
}

func (s *SurrealStore) Add(userId string, text string, vector []float32) error {
	item := SurrealMemoryItem{
		UserID:    userId,
		Text:      text,
		Vector:    vector,
		Timestamp: time.Now().Unix(),
	}

	_, err := s.client.Create("memories", item)
	return err
}

func (s *SurrealStore) Search(userId string, queryVector []float32, limit int) ([]string, error) {
	// Vector search query
	// Include 'vector' in SELECT since we're ordering by it
	query := `
		SELECT text, vector FROM memories
		WHERE user_id = $user_id
		ORDER BY vector <|` + fmt.Sprintf("%d", limit) + `|> $query_vector;
	`

	vars := map[string]interface{}{
		"user_id":      userId,
		"query_vector": queryVector,
	}

	result, err := s.client.Query(query, vars)
	if err != nil {
		return nil, err
	}

	// Parse result
	// SurrealDB returns an array of results, one for each query statement.
	// We executed one statement.
	resSlice, ok := result.([]interface{})
	if !ok || len(resSlice) == 0 {
		return []string{}, nil
	}

	// The first element is the result of our query
	queryRes := resSlice[0]

	// Handle different response formats (status/result object or direct array)
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

	var texts []string
	for _, row := range rows {
		if rowMap, ok := row.(map[string]interface{}); ok {
			if text, ok := rowMap["text"].(string); ok {
				texts = append(texts, text)
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

	// Cleanup old messages (keep last 10)
	// Changed to SELECT id, timestamp (not SELECT VALUE) since we need to order by timestamp
	query := `
		DELETE recent_messages
		WHERE user_id = $user_id
		AND id NOT IN (
			SELECT id, timestamp FROM recent_messages
			WHERE user_id = $user_id
			ORDER BY timestamp DESC
			LIMIT 10
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
