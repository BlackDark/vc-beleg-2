package main

import (
	"context"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/folderimport"
	"github.com/blackdark/vc-beleg-2/internal/mailimport"
)

func configureConnections(cfg *Config, a *app.App, reader app.Extractor) error {
	if ping, ok := reader.(interface{ Ping(context.Context) error }); ok {
		a.ConnectionChecks["reader"] = ping.Ping
	}
	if cfg.Mail != nil {
		mail := *cfg.Mail
		a.ConnectionChecks["mail"] = func(ctx context.Context) error { return mailimport.CheckConnection(ctx, mail) }
	}
	if cfg.Folder != nil {
		dav, err := folderimport.NewDav(cfg.Folder.URL, cfg.Folder.Username, cfg.Folder.Password, cfg.Folder.UserAgent)
		if err != nil {
			return err
		}
		a.ConnectionChecks["folder"] = func(ctx context.Context) error { _, err := dav.List(ctx); return err }
	}
	return nil
}
