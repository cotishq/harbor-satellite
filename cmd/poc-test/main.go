package main

import (
	"context"
	"fmt"
	"log"

	"github.com/container-registry/harbor-satellite/internal/satellite/state"
	"github.com/container-registry/harbor-satellite/pkg/config"
)

func main() {
	fmt.Println("=== Harbor Satellite P2P PoC ===")
	fmt.Println("Satellite B (localhost:8587) will pull from Satellite A (localhost:8586)")
	fmt.Println()

	cfg := config.PeerDistributionConfig{
		Enabled: true,
		Peers: []config.PeerConfig{
			{URL: "localhost:8586"},
		},
		Timeout:     "30s",
		MaxAttempts: 3,
		Backoff:     "2s",
		Concurrency: 1,
	}

	fallback := state.NewBasicReplicator("", "", "", "localhost:8587", "", "", true)
	replicator := state.NewPeerReplicator(cfg, fallback, "localhost:8587")

	entities := []state.Entity{
		{
			Name:       "alpine",
			Repository: "library",
			Tag:        "latest",
		},
	}

	fmt.Println("Starting peer transfer...")
	err := replicator.Replicate(context.Background(), entities)
	if err != nil {
		log.Fatalf("Transfer failed: %v", err)
	}

	fmt.Println()
	fmt.Println("=== Transfer complete! ===")
	fmt.Println("Verify with: curl http://localhost:8587/v2/_catalog")
}