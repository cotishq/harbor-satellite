package state

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/container-registry/harbor-satellite/internal/logger"
	"github.com/container-registry/harbor-satellite/pkg/config"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"oras.land/oras-go/v2"
	orasremote "oras.land/oras-go/v2/registry/remote"
)

const defaultPeerReplicatorTimeout = 30 * time.Second

type PeerReplicator struct {
	cfg      config.PeerDistributionConfig
	fallback Replicator
	localURL string
	client   *http.Client
	timeout  time.Duration
}

func NewPeerReplicator(cfg config.PeerDistributionConfig, fallback Replicator, localURL string) Replicator {
	timeout := peerTimeout(cfg.Timeout)
	return &PeerReplicator{
		cfg:      cfg,
		fallback: fallback,
		localURL: normalizeRegistryURL(localURL),
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

func (r *PeerReplicator) Replicate(ctx context.Context, replicationEntities []Entity) error {
	log := logger.FromContext(ctx)
	if !r.cfg.Enabled || len(r.cfg.Peers) == 0 {
		log.Debug().Bool("enabled", r.cfg.Enabled).Int("peers", len(r.cfg.Peers)).Msg("peer distribution disabled, using fallback replicator")
		return r.fallback.Replicate(ctx, replicationEntities)
	}

	remaining := make([]Entity, 0, len(replicationEntities))
	for _, entity := range replicationEntities {
		select {
		case <-ctx.Done():
			log.Warn().Err(ctx.Err()).Msg("context cancelled, stopping peer replication")
			return ctx.Err()
		default:
		}

		transferred := false
		for _, peer := range r.cfg.Peers {
			peerURL := strings.TrimSpace(peer.URL)
			if peerURL == "" {
				log.Warn().Str("repository", entity.GetRepository()).Str("name", entity.GetName()).Str("tag", entity.GetTag()).Msg("skipping peer with empty URL")
				continue
			}

			peerCtx, cancel := context.WithTimeout(ctx, r.timeout)
			if err := r.checkPeerLiveness(peerCtx, peerURL); err != nil {
				cancel()
				log.Warn().Err(err).Str("peer", peerURL).Msg("peer registry is not reachable")
				continue
			}

			if err := r.checkPeerArtifact(peerCtx, peerURL, entity); err != nil {
				cancel()
				log.Warn().Err(err).Str("peer", peerURL).Str("repository", entity.GetRepository()).Str("name", entity.GetName()).Str("tag", entity.GetTag()).Msg("artifact is not available from peer")
				continue
			}

			if err := r.copyFromPeer(peerCtx, peerURL, entity); err != nil {
				cancel()
				log.Warn().Err(err).Str("peer", peerURL).Str("repository", entity.GetRepository()).Str("name", entity.GetName()).Str("tag", entity.GetTag()).Msg("failed to transfer artifact from peer")
				continue
			}
			cancel()

			log.Info().Str("peer", peerURL).Str("repository", entity.GetRepository()).Str("name", entity.GetName()).Str("tag", entity.GetTag()).Msg("artifact replicated from peer")
			transferred = true
			break
		}

		if !transferred {
			remaining = append(remaining, entity)
		}
	}

	if len(remaining) == 0 {
		return nil
	}

	log.Info().Int("remaining", len(remaining)).Msg("falling back to upstream replication for artifacts unavailable from peers")
	return r.fallback.Replicate(ctx, remaining)
}

func (r *PeerReplicator) DeleteReplicationEntity(ctx context.Context, replicationEntity []Entity) error {
	return r.fallback.DeleteReplicationEntity(ctx, replicationEntity)
}

func (r *PeerReplicator) checkPeerLiveness(ctx context.Context, peerURL string) error {
	endpoint, err := peerV2URL(peerURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create peer liveness request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("check peer liveness: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		return nil
	}

	return fmt.Errorf("unexpected peer liveness status: %d", resp.StatusCode)
}

func (r *PeerReplicator) checkPeerArtifact(ctx context.Context, peerURL string, entity Entity) error {
	ref, err := name.ParseReference(entityRef(normalizeRegistryURL(peerURL), entity), name.Insecure)
	if err != nil {
		return fmt.Errorf("parse peer artifact reference: %w", err)
	}

	_, err = remote.Head(ref, remote.WithAuth(authn.Anonymous), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("head peer artifact: %w", err)
	}

	return nil
}

func (r *PeerReplicator) copyFromPeer(ctx context.Context, peerURL string, entity Entity) error {
	srcRepo, err := orasremote.NewRepository(entityRepository(normalizeRegistryURL(peerURL), entity))
	if err != nil {
		return fmt.Errorf("create peer repository: %w", err)
	}
	srcRepo.PlainHTTP = true

	dstRepo, err := orasremote.NewRepository(entityRepository(r.localURL, entity))
	if err != nil {
		return fmt.Errorf("create local repository: %w", err)
	}
	dstRepo.PlainHTTP = true

	_, err = oras.Copy(ctx, srcRepo, entity.GetTag(), dstRepo, entity.GetTag(), oras.CopyOptions{})
	if err != nil {
		return fmt.Errorf("copy artifact from peer: %w", err)
	}

	return nil
}

func peerTimeout(raw string) time.Duration {
	if raw == "" {
		return defaultPeerReplicatorTimeout
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return defaultPeerReplicatorTimeout
	}
	return timeout
}

func peerV2URL(peerURL string) (string, error) {
	if !strings.HasPrefix(peerURL, "http://") && !strings.HasPrefix(peerURL, "https://") {
		peerURL = "http://" + peerURL
	}
	parsed, err := url.Parse(peerURL)
	if err != nil {
		return "", fmt.Errorf("parse peer URL: %w", err)
	}
	parsed.Path = "/v2/"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func normalizeRegistryURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return strings.TrimRight(raw, "/")
}

func entityRef(registry string, entity Entity) string {
	return fmt.Sprintf("%s:%s", entityRepository(registry, entity), entity.GetTag())
}

func entityRepository(registry string, entity Entity) string {
	return fmt.Sprintf("%s/%s/%s", registry, entity.GetRepository(), entity.GetName())
}
