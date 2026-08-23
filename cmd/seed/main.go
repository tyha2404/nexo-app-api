package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/data"
	"github.com/tyha2404/nexo-app-api/internal/db"
	"github.com/tyha2404/nexo-app-api/internal/logger"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

func main() {
	forceFlag := flag.Bool("force", false, "Force re-embedding and overwrite all existing knowledge entries")
	flag.Parse()

	startTime := time.Now()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logg, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logg.Sync()

	gormDB, err := db.NewPostgres(cfg, logg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	knowledgeRepo := repository.NewKnowledgeRepository(gormDB)
	requestyService := service.NewRequestyService(cfg, logg)
	ragService := service.NewRAGService(knowledgeRepo, requestyService, logg)

	ctx := context.Background()

	fileDocs, err := data.LoadEmbeddedKnowledgeDocs()
	if err != nil {
		log.Fatalf("Failed to load embedded knowledge documents: %v", err)
	}

	fmt.Printf("\n🚀 Nexo Knowledge Base Seeder\n")
	fmt.Printf("------------------------------------------------------------------\n")
	fmt.Printf("Loaded %d knowledge files from internal/data/knowledge/\n", len(fileDocs))
	fmt.Printf("Force re-embed: %v | Requesty AI Configured: %v\n\n", *forceFlag, requestyService.IsConfigured())

	if *forceFlag {
		for _, doc := range fileDocs {
			if err := ragService.AddKnowledge(ctx, doc.Topic, doc.Title, doc.Content); err != nil {
				log.Fatalf("Failed to force seed doc %s: %v", doc.Topic, err)
			}
		}
	} else {
		if err := ragService.SeedDefaultKnowledge(ctx); err != nil {
			log.Fatalf("Knowledge seeding failed: %v", err)
		}
	}

	docs, err := knowledgeRepo.ListAll(ctx)
	if err != nil {
		log.Fatalf("Failed to list knowledge docs: %v", err)
	}

	fmt.Printf("%-3s | %-24s | %-45s | %s\n", "No.", "Topic", "Title", "Embedding Status")
	fmt.Printf("----+--------------------------+-----------------------------------------------+-----------------\n")
	for i, d := range docs {
		status := "✅ Valid Vector"
		if d.Embedding == "" || d.Embedding == "[]" || len(d.Embedding) < 10 {
			status = "❌ Empty"
		} else {
			status = fmt.Sprintf("✅ %d chars", len(d.Embedding))
		}
		title := d.Title
		if len([]rune(title)) > 43 {
			title = string([]rune(title)[:40]) + "..."
		}
		fmt.Printf("%-3d | %-24s | %-45s | %s\n", i+1, d.Topic, title, status)
	}

	fmt.Printf("\n✨ Seeding completed successfully in %v\n\n", time.Since(startTime))
}
