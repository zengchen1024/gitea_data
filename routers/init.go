// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routers

import (
	"context"
	"reflect"
	"runtime"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	authmodel "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/external"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/system"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/automerge"
	"code.gitea.io/gitea/services/cron"
	feed_service "code.gitea.io/gitea/services/feed"
	indexer_service "code.gitea.io/gitea/services/indexer"
	"code.gitea.io/gitea/services/mailer"
	mailer_incoming "code.gitea.io/gitea/services/mailer/incoming"
	markup_service "code.gitea.io/gitea/services/markup"
	repo_migrations "code.gitea.io/gitea/services/migrations"
	mirror_service "code.gitea.io/gitea/services/mirror"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/services/repository/archiver"
	"code.gitea.io/gitea/services/task"
	"code.gitea.io/gitea/services/uinotification"
	"code.gitea.io/gitea/services/webhook"
	web_routers "github.com/openmerlin/gitea_data/routers/web"
)

func mustInit(fn func() error) {
	err := fn()
	if err != nil {
		ptr := reflect.ValueOf(fn).Pointer()
		fi := runtime.FuncForPC(ptr)
		log.Fatal("%s failed: %v", fi.Name(), err)
	}
}

func mustInitCtx(ctx context.Context, fn func(ctx context.Context) error) {
	err := fn(ctx)
	if err != nil {
		ptr := reflect.ValueOf(fn).Pointer()
		fi := runtime.FuncForPC(ptr)
		log.Fatal("%s(ctx) failed: %v", fi.Name(), err)
	}
}

func syncAppConfForGit(ctx context.Context) error {
	runtimeState := new(system.RuntimeState)
	if err := system.AppState.Get(runtimeState); err != nil {
		return err
	}

	updated := false
	if runtimeState.LastAppPath != setting.AppPath {
		log.Info("AppPath changed from '%s' to '%s'", runtimeState.LastAppPath, setting.AppPath)
		runtimeState.LastAppPath = setting.AppPath
		updated = true
	}
	if runtimeState.LastCustomConf != setting.CustomConf {
		log.Info("CustomConf changed from '%s' to '%s'", runtimeState.LastCustomConf, setting.CustomConf)
		runtimeState.LastCustomConf = setting.CustomConf
		updated = true
	}

	if updated {
		log.Info("re-sync repository hooks ...")
		mustInitCtx(ctx, repo_service.SyncRepositoryHooks)

		log.Info("re-write ssh public keys ...")
		mustInitCtx(ctx, asymkey_model.RewriteAllPublicKeys)

		return system.AppState.Set(runtimeState)
	}
	return nil
}

func InitWebInstallPage(ctx context.Context) {
	translation.InitLocales(ctx)
	setting.LoadSettingsForInstall()
	mustInit(svg.Init)
}

// InitWebInstalled is for global installed configuration.
func InitWebInstalled(ctx context.Context) {
	mustInitCtx(ctx, git.InitFull)
	log.Info("Git version: %s (home: %s)", git.VersionInfo(), git.HomeDir())

	// Setup i18n
	translation.InitLocales(ctx)

	setting.LoadSettings()
	mustInit(storage.Init)

	mailer.NewContext(ctx)
	mustInit(cache.NewContext)
	mustInit(feed_service.Init)
	mustInit(uinotification.Init)
	mustInit(archiver.Init)

	highlight.NewContext()
	external.RegisterRenderers()
	markup.Init(markup_service.ProcessorHelper())

	if setting.EnableSQLite3 {
		log.Info("SQLite3 support is enabled")
	} else if setting.Database.Type.IsSQLite3() {
		log.Fatal("SQLite3 support is disabled, but it is used for database setting. Please get or build a Gitea release with SQLite3 support.")
	}

	mustInitCtx(ctx, common.InitDBEngine)
	log.Info("ORM engine initialization successful!")
	mustInit(system.Init)
	mustInit(oauth2.Init)

	mustInitCtx(ctx, models.Init)
	mustInitCtx(ctx, authmodel.Init)
	mustInitCtx(ctx, repo_service.Init)

	// Booting long running goroutines.
	mustInit(indexer_service.Init)

	mirror_service.InitSyncMirrors()
	mustInit(webhook.Init)
	mustInit(pull_service.Init)
	mustInit(automerge.Init)
	mustInit(task.Init)
	mustInit(repo_migrations.Init)
	eventsource.GetManager().Init()
	mustInitCtx(ctx, mailer_incoming.Init)

	mustInitCtx(ctx, syncAppConfForGit)

	mustInit(ssh.Init)

	auth.Init()
	mustInit(svg.Init)

	actions_service.Init()

	// Finally start up the cron
	cron.NewContext(ctx)
}

// NormalRoutes represents non install routes
func NormalRoutes() *web.Route {
	r := web.NewRoute()
	r.Use(common.ProtocolMiddlewares()...)

	r.Mount("/", web_routers.Routes())

	return r
}
