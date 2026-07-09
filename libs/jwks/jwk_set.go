package jwks

import (
	"context"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

func NewJWKSet(jwkUrl string) jwk.Set {
	client := httprc.NewClient()

	jwkCache, err := jwk.NewCache(context.Background(), client)
	if err != nil {
		panic("failed to create jwk cache: " + err.Error())
	}

	// Register with a 10s startup timeout; WaitReady(true) is the default so
	// this blocks until the first fetch completes, replacing the old Refresh call.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = jwkCache.Register(ctx, jwkUrl, jwk.WithMinInterval(10*time.Minute)); err != nil {
		panic("failed to register jwk location: " + err.Error())
	}

	cachedSet, err := jwkCache.CachedSet(jwkUrl)
	if err != nil {
		panic("failed to create cached jwk set: " + err.Error())
	}

	return cachedSet
}
