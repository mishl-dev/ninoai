package surreal

import (
	"context"
	"fmt"
	"log"

	"github.com/surrealdb/surrealdb.go"
)

type Client struct {
	db *surrealdb.DB
}

func NewClient(host, user, pass, namespace, database string) (*Client, error) {
	db, err := surrealdb.New(host)
	if err != nil {
		return nil, fmt.Errorf("failed to create surrealdb client: %w", err)
	}

	if _, err = db.SignIn(context.Background(), map[string]interface{}{
		"user": user,
		"pass": pass,
	}); err != nil {
		return nil, fmt.Errorf("failed to signin to surrealdb: %w", err)
	}

	if err = db.Use(context.Background(), namespace, database); err != nil {
		return nil, fmt.Errorf("failed to use surrealdb namespace/database: %w", err)
	}

	return &Client{db: db}, nil
}

func (c *Client) Close() {
	c.db.Close(context.Background())
}

func (c *Client) Query(sql string, vars interface{}) (interface{}, error) {
	result, err := surrealdb.Query[interface{}](context.Background(), c.db, sql, vars.(map[string]interface{}))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Create(thing string, data interface{}) (interface{}, error) {
	result, err := surrealdb.Create[interface{}](context.Background(), c.db, thing, data)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Select(thing string) (interface{}, error) {
	result, err := surrealdb.Select[interface{}](context.Background(), c.db, thing)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// VectorSearch performs a cosine similarity search on the specified table and vector field.
// It encapsulates the raw query construction to keep business logic clean.
func (c *Client) VectorSearch(table string, vectorField string, queryVector []float32, limit int, filter map[string]interface{}) ([]interface{}, error) {
	// Use SurrealDB's k-NN vector search syntax: <|limit,COSINE|>
	// This is the correct syntax according to SurrealDB documentation
	query := fmt.Sprintf("SELECT *, vector::distance::cosine(%s, $query_vector) AS distance FROM %s WHERE %s ORDER BY %s <|%d,COSINE|> $query_vector;",
		vectorField, table, buildWhereClause(filter), vectorField, limit)

	log.Printf("[DEBUG] VectorSearch query: %s", query)
	log.Printf("[DEBUG] Query vector length: %d, limit: %d", len(queryVector), limit)

	vars := map[string]interface{}{
		"query_vector": queryVector,
	}

	// Add filter variables
	for k, v := range filter {
		vars[k] = v
	}

	result, err := c.Query(query, vars)
	if err != nil {
		log.Printf("[DEBUG] VectorSearch error: %v", err)
		return nil, err
	}

	// Parse the result (array of results for each statement)
	resSlice, ok := result.([]interface{})
	if !ok || len(resSlice) == 0 {
		log.Printf("[DEBUG] No results or invalid format")
		return nil, nil
	}

	// The first element is the result of our query
	queryRes := resSlice[0]

	// Handle different response formats
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

	log.Printf("[DEBUG] VectorSearch returned %d rows", len(rows))
	return rows, nil
}

func buildWhereClause(filter map[string]interface{}) string {
	if len(filter) == 0 {
		return "true"
	}
	clause := ""
	i := 0
	for k := range filter {
		if i > 0 {
			clause += " AND "
		}
		clause += fmt.Sprintf("%s = $%s", k, k)
		i++
	}
	return clause
}
