package flyutil

import (
	"context"
	"github.com/superfly/fly-go"
	"github.com/superfly/flyctl/internal/cache"
	"github.com/superfly/flyctl/internal/logger"
	"time"
)

func FetchApp(ctx context.Context, client Client, name string) (*fly.AppCompact, error) {
	if client == nil {
		client = ClientFromContext(ctx)
	}
	c := cache.FromContext(ctx)
	if app := c.GetAppCompact(name); app != nil {
		logger.FromContext(ctx).Infof("Got app from cache")
		return app, nil
	} else {
		logger.FromContext(ctx).Infof("calling client")
		if app, err := client.GetAppCompact(ctx, name); err != nil {
			return nil, err
		} else {
			c.SetAppCompact(name, app)
			return app, nil
		}
	}

}

func FetchOrganizations(ctx context.Context, client Client) ([]*fly.OrganizationBasic, error) {
	if client == nil {
		client = ClientFromContext(ctx)
	}
	c := cache.FromContext(ctx)
	if o := c.GetOrganizations(); len(o) > 0 {
		return o, nil
	} else {
		if orgs, err := client.GetOrganizations(ctx); err != nil {
			return nil, err
		} else {
			var orgBasics []*fly.OrganizationBasic
			for _, org := range orgs {
				orgBasic := org.Basic()
				orgBasic.InternalNumericID = org.InternalNumericID
				orgBasics = append(orgBasics, orgBasic)
			}
			c.SetOrganizations(orgBasics)
			return orgBasics, nil
		}
	}
}

func FetchCertificate(ctx context.Context, cacheKey string, duration time.Duration, fn func() (*fly.IssuedCertificate, error)) (*fly.IssuedCertificate, error) {
	c := cache.FromContext(ctx)
	if o := c.GetCertificate(cacheKey); o != nil {
		return o, nil
	} else {
		cert, err := fn()
		if err != nil {
			return nil, err
		} else {
			c.SetCertificate(cacheKey, cert, duration)
			return cert, nil
		}
	}
}
