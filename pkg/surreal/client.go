package surreal

import (
	"context"
	"fmt"
	"log"
	"reflect"

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

// VectorSearch performs a cosine similarity search
func (c *Client) VectorSearch(table string, vectorField string, queryVector []float32, limit int, filter map[string]interface{}) ([]interface{}, error) {
	query := fmt.Sprintf(`
		SELECT *, vector::similarity::cosine(%s, $query_vector) AS similarity 
		FROM %s 
		WHERE %s 
		ORDER BY similarity DESC 
		LIMIT %d;
	`, vectorField, table, buildWhereClause(filter), limit)

	log.Printf("[DEBUG] VectorSearch query: %s", query)

	vars := map[string]interface{}{
		"query_vector": queryVector,
	}

	for k, v := range filter {
		vars[k] = v
	}

	result, err := c.Query(query, vars)
	if err != nil {
		log.Printf("[DEBUG] VectorSearch error: %v", err)
		return nil, err
	}

	log.Printf("[DEBUG] Raw result type: %T", result)

	// Use reflection to extract the Result field from QueryResult struct
	var rows []interface{}

	rv := reflect.ValueOf(result)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Slice && rv.Len() > 0 {
		firstElem := rv.Index(0)

		if firstElem.Kind() == reflect.Struct {
			resultField := firstElem.FieldByName("Result")
			if resultField.IsValid() && resultField.CanInterface() {
				if r, ok := resultField.Interface().([]interface{}); ok {
					rows = r
					log.Printf("[DEBUG] Successfully extracted %d rows via reflection", len(rows))
				}
			}
		}
	}

	log.Printf("[DEBUG] VectorSearch returning %d rows", len(rows))
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
