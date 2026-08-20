package serv

import (
	"net/http"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
)

// Placeholders for removed serv product surfaces.

type localKeystore struct{}

func (k *localKeystore) Close() {}

type configPreviewStore struct{}

func newConfigPreviewStore() *configPreviewStore { return &configPreviewStore{} }

func rateLimiter(_ *HttpService, h http.Handler) http.Handler { return h }

func startConfigWatcher(_ *HttpService) error { return nil }

func (s *graphjinService) initManagedArtifactStore() error { return nil }

func (s *graphjinService) hydrateCoreConfigSecrets(conf *core.Config) error { return nil }

func (s *graphjinService) hydrateLegacyDatabaseSecrets(db *Database) error { return nil }
