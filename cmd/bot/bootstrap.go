package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"SuperBotGo/internal/admin"
	adminapi "SuperBotGo/internal/admin/api"
	tsuauth "SuperBotGo/internal/auth/tsu"
	"SuperBotGo/internal/authz"
	"SuperBotGo/internal/authz/outbox"
	"SuperBotGo/internal/authz/providers"
	"SuperBotGo/internal/authz/tuples"
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/channel/dedup"
	"SuperBotGo/internal/channel/discord"
	"SuperBotGo/internal/channel/mattermost"
	"SuperBotGo/internal/channel/telegram"
	"SuperBotGo/internal/channel/vk"
	"SuperBotGo/internal/chat"
	"SuperBotGo/internal/config"
	"SuperBotGo/internal/database"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/core"
	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/university"
	"SuperBotGo/internal/user"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/eventbus"
	"SuperBotGo/internal/wasm/hostapi"
	"SuperBotGo/internal/wasm/registry"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type runtimeServices struct {
	adapterRegistry *channel.AdapterRegistry
	pluginManager   *plugin.Manager
	senderAPI       *plugin.SenderAPI
	metrics         *metrics.Metrics
	rt              *wasmrt.Runtime
	hostAPI         *hostapi.HostAPI
	wasmLoader      *adapter.Loader
	triggerRegistry *trigger.Registry
	triggerRouter   *trigger.Router
	cronScheduler   *trigger.CronScheduler
}

type postgresServices struct {
	pool               *pgxpool.Pool
	connString         string
	syncSvc            *university.SyncService
	userRepo           *user.PgUserRepo
	accountRepo        *user.PgAccountRepo
	pluginStore        *adminapi.PgPluginStore
	versionStore       *adminapi.PgVersionStore
	cmdPermStore       *adminapi.PgCommandPermStore
	adminChatStore     *adminapi.PgAdminChatStore
	authzStore         *authz.PgStore
	universityProvider *providers.UniversityProvider
	adminBus           *pubsub.Bus
	chatRegistry       chat.Registry
	notifPrefsRepo     *notification.PgPrefsRepo
}

type tsuAuthServices struct {
	stateStore *tsuauth.StateStore
	linker     core.TsuAuthLinker
}

func newLogger() *slog.Logger {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	return logger
}

func loadApplicationConfig(logger *slog.Logger) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	locale.SetDefault(cfg.DefaultLocale)
	if err := i18n.Init(cfg.DefaultLocale); err != nil {
		logger.Warn("i18n initialization failed, continuing with fallback keys", slog.Any("error", err))
	}

	return cfg, nil
}

func newFileStore(ctx context.Context, cfg *config.Config, logger *slog.Logger) (filestore.FileStore, error) {
	store, err := filestore.NewS3Store(ctx, filestore.S3StoreConfig{
		Bucket:    cfg.FileStore.S3.Bucket,
		Region:    cfg.FileStore.S3.Region,
		Endpoint:  cfg.FileStore.S3.Endpoint,
		AccessKey: cfg.FileStore.S3.AccessKey,
		SecretKey: cfg.FileStore.S3.SecretKey,
		Prefix:    cfg.FileStore.S3.Prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 file store: %w", err)
	}
	logger.Info("using S3 file store", slog.String("bucket", cfg.FileStore.S3.Bucket))
	return store, nil
}

func newRuntimeServices(ctx context.Context, cfg *config.Config, logger *slog.Logger, fileStore filestore.FileStore) (*runtimeServices, error) {
	services := &runtimeServices{
		adapterRegistry: channel.NewAdapterRegistry(),
		pluginManager:   plugin.NewManager(),
		metrics:         metrics.New(),
	}

	rt, err := wasmrt.NewRuntime(ctx, wasmrt.Config{
		CacheDir:                cfg.Admin.ModulesDir + "/.cache",
		DefaultMemoryLimitPages: defaultWasmMemoryLimitPages,
	})
	if err != nil {
		return nil, fmt.Errorf("create wasm runtime: %w", err)
	}
	rt.SetMetrics(services.metrics)
	services.rt = rt

	hostAPI := hostapi.NewHostAPI(hostapi.Dependencies{FileStore: fileStore})
	hostAPI.SetMetrics(services.metrics)
	hostAPI.SetEventBus(eventbus.New(nil, eventbus.NewMetrics()))
	logger.Info("plugin event bus initialised with at-least-once delivery")

	if err := hostAPI.RegisterHostModule(ctx, rt); err != nil {
		return nil, fmt.Errorf("register wasm host module: %w", err)
	}
	services.hostAPI = hostAPI

	pluginRegistry := registry.NewPluginRegistry()
	services.wasmLoader = adapter.NewLoader(rt, hostAPI, adapter.MessageSendFunc(func(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
		if services.senderAPI == nil {
			return fmt.Errorf("sender API not initialized")
		}
		return services.senderAPI.ReplyToChat(ctx, channelType, chatID, msg)
	}))
	services.wasmLoader.SetMetrics(services.metrics)
	services.wasmLoader.SetRegistry(pluginRegistry)

	services.triggerRegistry = trigger.NewRegistry()
	services.wasmLoader.SetTriggerRegistry(services.triggerRegistry)

	services.triggerRouter = trigger.NewRouter(services.triggerRegistry, services.pluginManager)
	services.cronScheduler = trigger.NewCronScheduler(services.triggerRouter)
	services.triggerRegistry.SetCronScheduler(services.cronScheduler)

	return services, nil
}

func newBlobStore(ctx context.Context, cfg *config.Config, logger *slog.Logger) (adminapi.BlobStore, error) {
	switch cfg.Admin.BlobStore {
	case "s3":
		store, err := adminapi.NewS3BlobStore(ctx, adminapi.S3BlobStoreConfig{
			Bucket:    cfg.Admin.S3.Bucket,
			Region:    cfg.Admin.S3.Region,
			Endpoint:  cfg.Admin.S3.Endpoint,
			AccessKey: cfg.Admin.S3.AccessKey,
			SecretKey: cfg.Admin.S3.SecretKey,
			Prefix:    cfg.Admin.S3.Prefix,
		})
		if err != nil {
			return nil, fmt.Errorf("create S3 blob store: %w", err)
		}
		logger.Info("using S3 blob store", slog.String("bucket", cfg.Admin.S3.Bucket))
		return store, nil
	default:
		store, err := adminapi.NewLocalFSBlobStore(cfg.Admin.ModulesDir)
		if err != nil {
			return nil, fmt.Errorf("create blob store: %w", err)
		}
		logger.Info("using local filesystem blob store", slog.String("dir", cfg.Admin.ModulesDir))
		return store, nil
	}
}

func newRedisClient(ctx context.Context, cfg *config.Config, logger *slog.Logger, cronScheduler *trigger.CronScheduler) (*redis.Client, error) {
	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("redis addr must be set")
	}
	client, err := database.NewRedisClient(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return nil, err
	}
	cronScheduler.SetRedis(client)
	logger.Info("connected to Redis", slog.String("addr", cfg.Redis.Addr))
	return client, nil
}

func newPostgresServices(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*postgresServices, error) {
	if cfg.Database.Host == "" || cfg.Database.DBName == "" {
		return nil, fmt.Errorf("database host and name must be set")
	}

	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	pool, err := database.NewPool(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := database.RunMigrations(connString); err != nil {
		return nil, fmt.Errorf("run database migrations: %w", err)
	}

	services := &postgresServices{
		pool:               pool,
		connString:         connString,
		syncSvc:            university.NewSyncService(pool),
		userRepo:           user.NewPgUserRepo(pool),
		accountRepo:        user.NewPgAccountRepo(pool),
		pluginStore:        adminapi.NewPgPluginStore(pool),
		versionStore:       adminapi.NewPgVersionStore(pool),
		cmdPermStore:       adminapi.NewPgCommandPermStore(pool),
		adminChatStore:     adminapi.NewPgAdminChatStore(pool),
		authzStore:         authz.NewPgStore(pool),
		universityProvider: providers.NewUniversityProvider(pool),
		adminBus:           pubsub.NewBus(pool, connString, generateInstanceID()),
		chatRegistry:       chat.NewPgRegistry(pool),
		notifPrefsRepo:     notification.NewPgPrefsRepo(pool),
	}

	logger.Info("using PostgreSQL stores")
	return services, nil
}

func configureSpiceDB(ctx context.Context, cfg *config.Config, services *postgresServices, logger *slog.Logger) (*authzed.Client, error) {
	if cfg.SpiceDB.Endpoint == "" {
		logger.Warn("SpiceDB endpoint not configured, authorization may not work correctly")
		return nil, nil
	}

	client, err := authzed.NewClient(
		cfg.SpiceDB.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpcutil.WithInsecureBearerToken(cfg.SpiceDB.Token),
	)
	if err != nil {
		return nil, fmt.Errorf("create SpiceDB client: %w", err)
	}
	logger.Info("SpiceDB client initialized", slog.String("endpoint", cfg.SpiceDB.Endpoint))

	schemaBytes, err := os.ReadFile("deployments/schema.zed")
	if err != nil {
		return nil, fmt.Errorf("read SpiceDB schema file: %w", err)
	}
	if _, err := client.WriteSchema(ctx, &v1.WriteSchemaRequest{Schema: string(schemaBytes)}); err != nil {
		return nil, fmt.Errorf("write SpiceDB schema: %w", err)
	}
	logger.Info("SpiceDB schema loaded")

	tupleWriter := tuples.NewWriter(client)
	outboxWorker := outbox.NewWorker(services.pool, tupleWriter, logger)
	go func() {
		if err := outboxWorker.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("authz outbox worker stopped", slog.Any("error", err))
		}
	}()

	return client, nil
}

func configureTSUAccounts(cfg *config.Config, userRepo *user.PgUserRepo, accountRepo *user.PgAccountRepo, pool *pgxpool.Pool, adminMux *http.ServeMux, logger *slog.Logger) tsuAuthServices {
	var services tsuAuthServices
	if cfg.TsuAccounts.ApplicationID == "" || cfg.TsuAccounts.SecretKey == "" {
		logger.Info("TSU.Accounts not configured, skipping")
		return services
	}

	tsuClient := tsuauth.NewClient(
		&http.Client{Timeout: 10 * time.Second},
		cfg.TsuAccounts.ApplicationID,
		cfg.TsuAccounts.SecretKey,
		cfg.TsuAccounts.BaseURL,
	)

	loginURL := fmt.Sprintf("http://localhost:%d/oauth/authorize", cfg.Admin.Port)
	if cfg.TsuAccounts.CallbackURL != "" {
		loginURL = strings.TrimSuffix(cfg.TsuAccounts.CallbackURL, "/login") + "/authorize"
	}
	services.stateStore = tsuauth.NewStateStore(loginURL)

	personLinker := user.NewPersonAutoLinker(pool)
	tsuLinker := tsuauth.NewLinker(userRepo, accountRepo, personLinker, logger)
	tsuHandler := tsuauth.NewHandler(tsuClient, services.stateStore, tsuLinker, cfg.TsuAccounts.CallbackURL, logger)
	tsuHandler.RegisterRoutes(adminMux)

	services.linker = services.stateStore
	logger.Info("TSU.Accounts authentication enabled")
	return services
}

func registerAdminRoutes(
	cfg *config.Config,
	logger *slog.Logger,
	runtime *runtimeServices,
	stores *postgresServices,
	blobStore adminapi.BlobStore,
	authorizer *authz.Authorizer,
	stateMgr *state.Manager,
	spiceClient *authzed.Client,
) (*http.ServeMux, *adminapi.AuthHandler) {
	adminHandler := adminapi.NewAdminHandler(
		stores.pluginStore,
		blobStore,
		runtime.wasmLoader,
		runtime.pluginManager,
		runtime.rt,
		runtime.hostAPI,
		stateMgr,
		stores.cmdPermStore,
		stores.versionStore,
		stores.adminBus,
		authorizer,
	)

	adminMux := http.NewServeMux()
	adminCredStore := adminapi.NewPgAdminCredStore(stores.pool)
	authHandler := adminapi.NewAuthHandler(cfg.Admin.APIKey, adminCredStore)
	authHandler.RegisterRoutes(adminMux)
	adminapi.NewAdminCredHandler(adminCredStore).RegisterRoutes(adminMux)
	adminHandler.RegisterRoutes(adminMux)
	adminapi.NewCommandPermHandler(stores.cmdPermStore, authorizer).RegisterRoutes(adminMux)
	adminapi.NewUserHandler(adminapi.NewPgUserStore(stores.pool), authorizer).RegisterRoutes(adminMux)
	adminapi.NewPluginPermHandler(stores.pluginStore, runtime.wasmLoader, runtime.hostAPI, stores.adminBus).RegisterRoutes(adminMux)
	adminapi.NewChatHandler(stores.adminChatStore, runtime.adapterRegistry).RegisterRoutes(adminMux)
	adminapi.NewChannelStatusHandler(runtime.adapterRegistry, adminapi.ChannelStatusConfig{
		TelegramConfigured:   cfg.Telegram.Token != "",
		DiscordConfigured:    cfg.Discord.Token != "",
		VKConfigured:         cfg.VK.Token != "",
		MattermostConfigured: cfg.Mattermost.URL != "" && cfg.Mattermost.Token != "",
	}).RegisterRoutes(adminMux)
	adminapi.NewRuleSchemaHandler(authz.NewRuleSchemaBuilder(stores.authzStore, stores.universityProvider)).RegisterRoutes(adminMux)
	if spiceClient != nil {
		adminapi.NewRelationshipHandler(spiceClient).RegisterRoutes(adminMux)
	}

	adminapi.NewUniversityRefHandler(stores.pool).RegisterRoutes(adminMux)
	positionStore := adminapi.NewPgPositionStore(stores.pool)
	adminapi.NewPositionHandler(positionStore).RegisterRoutes(adminMux)
	adminapi.NewImportHandler(stores.syncSvc).RegisterRoutes(adminMux)
	adminapi.NewUniversitySyncHandler(stores.syncSvc).RegisterRoutes(adminMux)

	httpTrigger := trigger.NewHTTPTriggerHandler(runtime.triggerRouter, runtime.triggerRegistry)
	httpTrigger.SetMetrics(runtime.metrics)
	adminMux.Handle("/api/triggers/http/", httpTrigger)
	adminMux.Handle("GET /metrics", promhttp.Handler())
	admin.RegisterStaticRoutes(adminMux)

	return adminMux, authHandler
}

func newAdminServer(cfg *config.Config, authHandler *adminapi.AuthHandler, mux *http.ServeMux) *http.Server {
	authMiddleware := adminapi.NewAdminAuthMiddleware(authHandler)
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Admin.Port),
		Handler:      authMiddleware.Wrap(mux),
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}

func startAdminServer(server *http.Server, logger *slog.Logger, port int) {
	go func() {
		logger.Info("starting Admin API HTTP server", slog.Int("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("admin API server error", slog.Any("error", err))
		}
	}()
}

func autoloadPlugins(ctx context.Context, stores *postgresServices, blobStore adminapi.BlobStore, runtime *runtimeServices, logger *slog.Logger) {
	if err := adminapi.AutoloadPlugins(ctx, stores.pluginStore, blobStore, runtime.wasmLoader, runtime.pluginManager); err != nil {
		logger.Warn("wasm autoload failed", slog.Any("error", err))
	}
	adapter.RegisterWasmPlugins(runtime.pluginManager, runtime.wasmLoader)
}

func startUniversityPuller(ctx context.Context, cfg *config.Config, syncSvc *university.SyncService, logger *slog.Logger) {
	if !cfg.UniversitySync.Enabled {
		return
	}

	pullInterval, err := time.ParseDuration(cfg.UniversitySync.Interval)
	if err != nil {
		pullInterval = defaultSyncInterval
	}
	dataSource := &university.StubDataSource{
		BaseURL: cfg.UniversitySync.BaseURL,
		Token:   cfg.UniversitySync.Token,
	}
	puller := university.NewPuller(dataSource, syncSvc, logger, pullInterval)
	go func() {
		if err := puller.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("university puller stopped", slog.Any("error", err))
		}
	}()
}

func registerPluginCommands(stateMgr *state.Manager, plugins []plugin.Plugin) {
	for _, p := range plugins {
		for _, def := range p.Commands() {
			stateMgr.RegisterCommand(p.ID(), def)
		}
	}
}

func registerPluginCommandsFromMap(stateMgr *state.Manager, plugins map[string]plugin.Plugin) {
	for _, p := range plugins {
		for _, def := range p.Commands() {
			stateMgr.RegisterCommand(p.ID(), def)
		}
	}
}

func collectCommandNames(manager *plugin.Manager) []string {
	var commandNames []string
	for _, p := range manager.All() {
		for _, def := range p.Commands() {
			commandNames = append(commandNames, def.Name)
		}
	}
	return commandNames
}

func startPubSubSubscriber(ctx context.Context, logger *slog.Logger, stores *postgresServices, blobStore adminapi.BlobStore, runtime *runtimeServices, stateMgr *state.Manager) {
	pluginFetcher := func(fCtx context.Context, id string) (*pubsub.PluginData, error) {
		rec, err := stores.pluginStore.GetPlugin(fCtx, id)
		if err != nil {
			return nil, err
		}
		return &pubsub.PluginData{
			WasmKey:    rec.WasmKey,
			ConfigJSON: rec.ConfigJSON,
		}, nil
	}
	eventHandler := pubsub.NewAdminEventHandler(pluginFetcher, blobStore.Get, runtime.wasmLoader, runtime.pluginManager, runtime.hostAPI, stateMgr)
	go func() {
		if err := stores.adminBus.Subscribe(ctx, eventHandler.Handle); err != nil {
			logger.Error("pubsub subscriber stopped", slog.Any("error", err))
		}
	}()
	logger.Info("pub/sub subscriber started", slog.String("instance", stores.adminBus.InstanceID()))
}

func registerBotFeatures(bot any, mux *http.ServeMux, commandNames []string) error {
	if registrar, ok := bot.(channel.CommandRegistrar); ok {
		registrar.RegisterCommands(commandNames)
	}
	if registrar, ok := bot.(channel.RouteRegistrar); ok {
		return registrar.RegisterRoutes(mux)
	}
	return nil
}

type botStarter func(context.Context)

func prepareConfiguredBots(
	cfg *config.Config,
	logger *slog.Logger,
	fileStore filestore.FileStore,
	redisClient *redis.Client,
	manager *channel.ChannelManager,
	commandNames []string,
	chatRegistry chat.Registry,
	mux *http.ServeMux,
) []botStarter {
	joinHandler := newChatJoinHandler(chatRegistry, logger)
	dedupMw := dedup.Middleware(redisClient, dedup.Config{}, logger)
	var starters []botStarter

	if cfg.Telegram.Token != "" {
		logger.Info("starting Telegram bot", slog.String("mode", cfg.Telegram.Mode))
		tgHandler := channel.Chain(manager.OnUpdate, dedupMw, telegram.CallbackNormalizer())
		tgBot, err := telegram.NewBot(telegram.BotConfig{
			Token:         cfg.Telegram.Token,
			Mode:          cfg.Telegram.Mode,
			WebhookURL:    cfg.Telegram.WebhookURL,
			WebhookSecret: cfg.Telegram.WebhookSecret,
			WebhookListen: cfg.Telegram.WebhookListen,
		}, tgHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
		if err != nil {
			logger.Error("failed to create Telegram bot", slog.Any("error", err))
		} else {
			if err := registerBotFeatures(tgBot, mux, commandNames); err != nil {
				logger.Error("failed to register Telegram features", slog.Any("error", err))
			} else {
				manager.RegisterAdapter(tgBot.Adapter())
				starters = append(starters, func(ctx context.Context) {
					if err := tgBot.Start(ctx); err != nil {
						logger.Error("Telegram bot stopped with error", slog.Any("error", err))
					}
				})
			}
		}
	} else {
		logger.Warn("Telegram token not configured, Telegram bot will not start")
	}

	if cfg.Discord.Token == "" {
		logger.Warn("Discord token not configured, Discord bot will not start")
	} else {
		logger.Info("starting Discord bot",
			slog.Int("shard_id", cfg.Discord.ShardID),
			slog.Int("shard_count", cfg.Discord.ShardCount))
		dcHandler := channel.Chain(manager.OnUpdate, dedupMw)
		dcBot, err := discord.NewBot(discord.BotConfig{
			Token:      cfg.Discord.Token,
			ShardID:    cfg.Discord.ShardID,
			ShardCount: cfg.Discord.ShardCount,
		}, dcHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
		if err != nil {
			logger.Error("failed to create Discord bot", slog.Any("error", err))
		} else {
			manager.RegisterAdapter(dcBot.Adapter())
			starters = append(starters, func(ctx context.Context) {
				if err := dcBot.Start(ctx); err != nil {
					logger.Error("Discord bot stopped with error", slog.Any("error", err))
				}
			})
		}
	}

	if cfg.VK.Token == "" {
		logger.Warn("VK token not configured, VK bot will not start")
	} else {
		logger.Info("starting VK bot", slog.String("mode", cfg.VK.Mode))
		vkHandler := channel.Chain(manager.OnUpdate, dedupMw)
		vkBot, err := vk.NewBot(vk.BotConfig{
			Token:        cfg.VK.Token,
			Mode:         cfg.VK.Mode,
			CallbackURL:  cfg.VK.CallbackURL,
			CallbackPath: cfg.VK.CallbackPath,
		}, vkHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
		if err != nil {
			logger.Error("failed to create VK bot", slog.Any("error", err))
		} else {
			if err := registerBotFeatures(vkBot, mux, commandNames); err != nil {
				logger.Error("failed to register VK features", slog.Any("error", err))
			} else {
				manager.RegisterAdapter(vkBot.Adapter())
				starters = append(starters, func(ctx context.Context) {
					if err := vkBot.Start(ctx); err != nil {
						logger.Error("VK bot stopped with error", slog.Any("error", err))
					}
				})
			}
		}
	}

	if cfg.Mattermost.URL == "" || cfg.Mattermost.Token == "" {
		logger.Warn("Mattermost config not complete, Mattermost bot will not start")
		return starters
	}

	logger.Info("starting Mattermost bot", slog.String("url", cfg.Mattermost.URL))
	mmHandler := channel.Chain(manager.OnUpdate, dedupMw)
	mmBot, err := mattermost.NewBot(mattermost.BotConfig{
		URL:           cfg.Mattermost.URL,
		Token:         cfg.Mattermost.Token,
		ActionsURL:    cfg.Mattermost.ActionsURL,
		ActionsPath:   cfg.Mattermost.ActionsPath,
		ActionsSecret: cfg.Mattermost.ActionsSecret,
	}, mmHandler, joinHandler, fileStore, cfg.FileStore.MaxFileSize, logger)
	if err != nil {
		logger.Error("failed to create Mattermost bot", slog.Any("error", err))
		return starters
	}

	if err := registerBotFeatures(mmBot, mux, commandNames); err != nil {
		logger.Error("failed to register Mattermost features", slog.Any("error", err))
		return starters
	}
	manager.RegisterAdapter(mmBot.Adapter())
	starters = append(starters, func(ctx context.Context) {
		if err := mmBot.Start(ctx); err != nil {
			logger.Error("Mattermost bot stopped with error", slog.Any("error", err))
		}
	})

	return starters
}

func startPreparedBots(ctx context.Context, starters []botStarter) {
	for _, starter := range starters {
		go starter(ctx)
	}
}

func startFileStoreCleanup(ctx context.Context, logger *slog.Logger, fileStore filestore.FileStore) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := fileStore.Cleanup(ctx)
				if err != nil {
					logger.Error("file store cleanup error", slog.Any("error", err))
				} else if n > 0 {
					logger.Info("file store cleanup", slog.Int("removed", n))
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func shutdownRuntime(ctx context.Context, logger *slog.Logger, adminServer *http.Server, cronScheduler *trigger.CronScheduler, tsuStateStore *tsuauth.StateStore, wasmLoader *adapter.Loader, rt *wasmrt.Runtime) {
	if err := adminServer.Shutdown(ctx); err != nil {
		logger.Error("admin API server shutdown error", slog.Any("error", err))
	} else {
		logger.Info("admin API server stopped")
	}

	cronScheduler.Stop()
	if tsuStateStore != nil {
		tsuStateStore.Stop()
	}
	if err := wasmLoader.Close(ctx); err != nil {
		logger.Error("wasm loader close error", slog.Any("error", err))
	}
	if err := rt.Close(ctx); err != nil {
		logger.Error("wasm runtime close error", slog.Any("error", err))
	}
}
