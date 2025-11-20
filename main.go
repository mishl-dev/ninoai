package main

import (
	"log"
	"ninoai/pkg/bot"
	"ninoai/pkg/cerebras"
	"ninoai/pkg/config"
	"ninoai/pkg/embedding"
	"ninoai/pkg/memory"
	"ninoai/pkg/surreal"

	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	// Load config.yml
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load .env for secrets
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	token := os.Getenv("DISCORD_TOKEN")
	cerebrasKey := os.Getenv("CEREBRAS_API_KEY")
	embeddingKey := os.Getenv("EMBEDDING_API_KEY")

	if token == "" || cerebrasKey == "" || embeddingKey == "" {
		log.Fatal("Missing required environment variables (DISCORD_TOKEN, CEREBRAS_API_KEY, EMBEDDING_API_KEY)")
	}

	// Initialize Clients
	cerebrasClient := cerebras.NewClient(cerebrasKey, cfg.ModelSettings.Temperature, cfg.ModelSettings.TopP)
	embeddingClient := embedding.NewClient(embeddingKey, cfg.EmbeddingAPIURL)

	// Initialize Memory Store (SurrealDB)
	surrealHost := os.Getenv("SURREAL_DB_HOST")
	surrealUser := os.Getenv("SURREAL_DB_USER")
	surrealPass := os.Getenv("SURREAL_DB_PASS")

	if surrealHost == "" || surrealUser == "" || surrealPass == "" {
		log.Fatal("Missing required environment variables for SurrealDB (SURREAL_DB_HOST, SURREAL_DB_USER, SURREAL_DB_PASS)")
	}

	// Add protocol if missing
	if len(surrealHost) > 0 && surrealHost[:4] != "ws://" && surrealHost[:5] != "wss://" {
		surrealHost = "wss://" + surrealHost + "/rpc"
	}

	log.Printf("Connecting to SurrealDB at %s", surrealHost)
	surrealClient, err := surreal.NewClient(surrealHost, surrealUser, surrealPass, "nino", "memory")
	if err != nil {
		log.Fatalf("Failed to connect to SurrealDB: %v", err)
	}
	defer surrealClient.Close()

	memoryStore := memory.NewSurrealStore(surrealClient)

	// Initialize Bot Handler
	handler := bot.NewHandler(cerebrasClient, embeddingClient, memoryStore, cfg.Delays.MessageProcessing)

	// Create Discord Session
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	// Register Handlers
	dg.AddHandler(handler.MessageCreate)
	dg.AddHandler(handler.InteractionCreate)

	// Open Connection
	if err := dg.Open(); err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}

	// Set Bot ID in handler (so it can ignore itself)
	handler.SetBotID(dg.State.User.ID)

	// Register slash commands (empty string = global, or specify guild ID for faster testing)
	// For production, use "" for global commands. For development, use a specific guild ID for instant updates.
	guildID := os.Getenv("DISCORD_GUILD_ID") // Optional: set this for faster command updates during development
	registeredCommands, err := bot.RegisterSlashCommands(dg, guildID)
	if err != nil {
		log.Fatalf("Error registering slash commands: %v", err)
	}

	// Cleanup function to unregister commands on shutdown
	defer func() {
		if err := bot.UnregisterSlashCommands(dg, guildID, registeredCommands); err != nil {
			log.Printf("Error unregistering slash commands: %v", err)
		}
	}()

	log.Println("Nino is now running. Press CTRL-C to exit.")

	// Set Custom Status
	err = dg.UpdateStatusComplex(discordgo.UpdateStatusData{
		Activities: []*discordgo.Activity{
			{
				Name:  "Custom Status",
				Type:  discordgo.ActivityTypeCustom,
				State: "someone has to keep this family alive.",
				Emoji: discordgo.Emoji{
					Name: "😔",
				},
			},
		},
		Status: "online",
		AFK:    true,
	})
	if err != nil {
		log.Printf("Error setting custom status: %v", err)
	}

	// Wait for signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}
