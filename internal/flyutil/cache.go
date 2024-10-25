package flyutil

import (
	"context"

	"github.com/superfly/fly-go"
	"github.com/superfly/flyctl/internal/cache"
)

func FetchAppBasic(ctx context.Context, name string) (*fly.AppBasic, error) {
	c := cache.FromContext(ctx)
	if app := c.GetAppBasic(name); app != nil {
		return app, nil
	} else {
		if app, err := ClientFromContext(ctx).GetAppBasic(ctx, name); err != nil {
			return nil, err
		} else {
			c.SetAppBasic(name, app)
			return app, nil
		}
	}

}

func FetchOrganizations(ctx context.Context) ([]*fly.OrganizationBasic, error) {
	c := cache.FromContext(ctx)
	if o := c.GetOrganizations(); o != nil {
		return o, nil
	} else {
		if orgs, err := ClientFromContext(ctx).GetOrganizations(ctx); err != nil {
			return nil, err
		} else {
			var orgBasics []*fly.OrganizationBasic
			for _, org := range orgs {
				orgBasics = append(orgBasics, org.Basic())
			}
			c.SetOrganizations(orgBasics)
			return orgBasics, nil
		}
	}
}
