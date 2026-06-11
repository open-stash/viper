package scrapingant

import (
	"context"

	clientpkg "github.com/open-stash/viper/pkg/scrapingant"
)

// Adapter wires the ScrapingAnt client to the engine's ProxyFetcher port.
type Adapter struct {
	client *clientpkg.Client
}

func NewAdapter(client *clientpkg.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) Fetch(ctx context.Context, url string, residential, jsRender bool) ([]byte, error) {
	return a.client.Fetch(ctx, url, clientpkg.Options{
		Residential: residential,
		JSRender:    jsRender,
	})
}
