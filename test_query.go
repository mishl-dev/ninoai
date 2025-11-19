package main

import (
	"fmt"
	"log"
	"ninoai/pkg/surreal"
	"os"
)

func main() {
	host := os.Getenv("SURREAL_DB_HOST")
	user := os.Getenv("SURREAL_DB_USER")
	pass := os.Getenv("SURREAL_DB_PASS")

	client, err := surreal.NewClient(host, user, pass, "nino", "memory")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Test query - just get all memories for the user
	query := `SELECT * FROM memories WHERE user_id = $user_id LIMIT 5;`
	result, err := client.Query(query, map[string]interface{}{
		"user_id": "1025245410224263258",
	})
	if err != nil {
		log.Fatalf("Query error: %v", err)
	}

	fmt.Printf("Result type: %T\n", result)
	fmt.Printf("Result: %+v\n", result)
}
