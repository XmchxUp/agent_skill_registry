package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"agent_skill_registry/internal/registry"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data", "data", "registry data directory")
	signingKey := flag.String("signing-key", "", "development signing key; defaults to ADP_SKILL_SIGNING_KEY or a local dev key")
	flag.Parse()

	key := *signingKey
	if key == "" {
		key = os.Getenv("ADP_SKILL_SIGNING_KEY")
	}
	if key == "" {
		key = "dev-z-root-signing-key"
	}

	service, err := registry.NewService(*dataDir, key)
	if err != nil {
		log.Fatalf("init service: %v", err)
	}

	mux := http.NewServeMux()
	registry.RegisterHandlers(mux, service)

	log.Printf("Agent Skill Registry MVP listening on http://localhost%s", *addr)
	log.Printf("data directory: %s", *dataDir)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
