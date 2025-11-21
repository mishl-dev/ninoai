# NinoAI 🎀

A Discord bot powered by AI that embodies the personality of Nino Nakano from "The Quintessential Quintuplets". NinoAI features advanced memory capabilities using SurrealDB for vector search and long-term memory retention.

[![License: Curse of Knowledge](https://img.shields.io/badge/License-Curse%20of%20Knowledge-red.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24.5-00ADD8?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://www.docker.com/)
[![Go Test](https://github.com/mishl-dev/ninoai/actions/workflows/test.yml/badge.svg)](https://github.com/mishl-dev/ninoai/actions/workflows/test.yml)

## ✨ Features

- **AI-Powered Conversations**: Uses Cerebras AI for natural language processing
- **Long-Term Memory**: Intelligent memory system using SurrealDB with vector search
- **Memory Agent**: AI-powered agent that decides which interactions are worth remembering
- **Rolling Context Window**: Maintains recent conversation context for coherent responses
- **Slash Commands**: Interactive commands for memory management and bot control
- **Character Personality**: Responds with Nino Nakano's distinctive personality and mannerisms
- **Expressive Responses**: Supports Discord emojis and GIFs for enhanced expressiveness

## 🏗️ Architecture

NinoAI uses a sophisticated memory flow:

1. **User Message** → Received via Discord
2. **Vector Search** → Searches SurrealDB for relevant long-term memories
3. **Context Assembly** → Combines retrieved memories with rolling chat context
4. **LLM Response** → Generates response using Cerebras AI
5. **Memory Evaluation** → AI agent decides if interaction should be stored long-term
6. **Storage** → Important memories are embedded and stored in SurrealDB

## 📋 Prerequisites

- **Go 1.24.5** or higher
- **SurrealDB** instance ([local or hosted](https://surrealdb.com/install))
- **Discord Bot Token** ([Create one here](https://discord.com/developers/applications))
- **Cerebras API Key** ([Get one here](https://cerebras.ai/))
- **Embedding API ([mishl-dev/text-embed-api](https://github.com/mishl-dev/text-embed-api))** ([hosted](https://github.com/mishl-dev/text-embed-api/))
- **HUGGGINGFACE API KEY** ([Create one here](https://huggingface.co/settings/tokens/new))

## 🚀 Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/mishl-dev/ninoai.git
cd ninoai
```

### 2. Set Up Environment Variables

Copy the example environment file and fill in your credentials:

```bash
cp example.env .env
```

Edit `.env` with your actual values:

```env
DISCORD_TOKEN=your_discord_bot_token
CEREBRAS_API_KEY=your_cerebras_api_key
EMBEDDING_API_URL=https://your-embedding-api.com/embed
EMBEDDING_API_KEY=your_embedding_api_key
SURREAL_DB_HOST=your-surrealdb-host.com
SURREAL_DB_USER=your_surrealdb_username
SURREAL_DB_PASS=your_surrealdb_password
HF_API_KEY=your_huggingface_api_key
```

### 3. Install Dependencies

```bash
go mod download
```

### 4. Run the Bot

```bash
go run main.go
```

## 🐳 Docker Deployment

### Using Docker Compose (Recommended)

The easiest way to run NinoAI with all dependencies:

1. **Set up environment variables**:
   ```bash
   cp example.env .env
   # Edit .env with your credentials
   ```

2. **Start the services**:
   ```bash
   docker-compose up -d
   ```

3. **View logs**:
   ```bash
   docker-compose logs -f ninoai
   ```

4. **Stop the services**:
   ```bash
   docker-compose down
   ```

### Using Docker Only

If you already have a SurrealDB instance:

1. **Build the image**:
   ```bash
   docker build -t ninoai:latest .
   ```

2. **Run the container**:
   ```bash
   docker run -d \
     --name ninoai \
     --env-file .env \
     -v $(pwd)/storage:/app/storage \
     ninoai:latest
   ```

### Docker Configuration

The `docker-compose.yml` includes:
- **SurrealDB**: Automatically configured and networked
- **Health Checks**: Ensures services start in the correct order
- **Persistent Storage**: Data persists across container restarts
- **Log Rotation**: Prevents log files from growing indefinitely
- **Automatic Restart**: Services restart unless manually stopped

To use an external SurrealDB instance, modify the `SURREAL_DB_HOST` in your `.env` file and remove the `surrealdb` service from `docker-compose.yml`.

## 🔧 Configuration

### Environment Variables

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `DISCORD_TOKEN` | ✅ | Discord bot token | - |
| `CEREBRAS_API_KEY` | ✅ | Cerebras AI API key | - |
| `EMBEDDING_API_KEY` | ✅ | API key for embedding service | - |
| `EMBEDDING_API_URL` | ❌ | Embedding API endpoint | `https://vector.mishl.dev/embed` |
| `SURREAL_DB_HOST` | ✅ | SurrealDB host (WebSocket) | - |
| `SURREAL_DB_USER` | ✅ | SurrealDB username | - |
| `SURREAL_DB_PASS` | ✅ | SurrealDB password | - |

### SurrealDB Setup

NinoAI uses SurrealDB with the following configuration:
- **Namespace**: `nino`
- **Database**: `memory`
- **Table**: `memories` (auto-created)

The bot automatically creates the necessary schema on first run.

## 🎮 Usage

### Slash Commands

- `/memory` - View your current memory statistics and manage stored memories
- `/resent` - Resend the last bot response (useful if a message was deleted)

### Interacting with the Bot

Simply mention the bot or send it a direct message! NinoAI will:
- Always respond in DMs
- Respond when mentioned in servers
- Remember important conversations
- Use context from previous interactions

## 🏗️ Project Structure

```
ninoai/
├── main.go                 # Application entry point
├── pkg/
│   ├── bot/               # Discord bot handlers and commands
│   ├── cerebras/          # Cerebras AI client
│   ├── embedding/         # Embedding API client
│   ├── memory/            # Memory management and storage
│   └── surreal/           # SurrealDB client wrapper
├── storage/               # Local storage directory
├── .env                   # Environment variables (not in git)
├── example.env            # Example environment configuration
└── go.mod                 # Go module dependencies
```

## 🧪 Testing

Run the test suite:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

## 🔨 Building

Build the executable:

```bash
go build -o ninoai
```

For production builds with optimizations:

```bash
go build -ldflags="-s -w" -o ninoai
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📝 License

This project is licensed under **The Curse of Knowledge License v1.0** - see the [LICENSE](LICENSE) file for details.

**⚠️ WARNING**: This is a paradoxical license. By reading the license text, you forfeit all rights granted by it. The license is essentially a humorous take on public domain dedication with a self-referential twist. If you haven't read the license yet, you have unlimited rights to use this work. If you have read it... well, it's too late now! 

For practical purposes, consider this project as freely available for use, modification, and distribution. The license is meant to be entertaining rather than legally restrictive.

## 🙏 Acknowledgments

- **The Quintessential Quintuplets** by Negi Haruba - for the amazing character
- **Cerebras AI** - for the powerful language model API
- **SurrealDB** - for the innovative database with built-in vector search
- **DiscordGo** - for the excellent Discord API wrapper

## 📧 Contact

For questions, issues, or suggestions, please open an issue on GitHub.
