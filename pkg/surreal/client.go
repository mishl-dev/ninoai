package surreal

import (
	"context"
	"fmt"
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
	result, err := surrealdb.Query[[]interface{}](context.Background(), c.db, sql, vars.(map[string]interface{}))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Create(thing string, data interface{}) (interface{}, error) {
	result, err := surrealdb.Create[[]interface{}](context.Background(), c.db, thing, data)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Select(thing string) (interface{}, error) {
	result, err := surrealdb.Select[[]interface{}](context.Background(), c.db, thing)
	if err != nil {
		return nil, err
	}
	return result, nil
}
