package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"SuperBotGo/internal/admin"
	adminapi "SuperBotGo/internal/admin/api"
	"SuperBotGo/internal/authz"
	"SuperBotGo/internal/authz/providers"
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/channel/discord"
	"SuperBotGo/internal/channel/telegram"
	"SuperBotGo/internal/chat"
	"SuperBotGo/internal/config"
	"SuperBotGo/internal/database"
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/broadcast"
	pluginLink "SuperBotGo/internal/plugin/link"
	"SuperBotGo/internal/plugin/project"
	"SuperBotGo/internal/plugin/resume"
	"SuperBotGo/internal/plugin/settings"
	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/role"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/state/storage"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/user"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	if err := i18n.Init(cfg.DefaultLocale); err != nil {
		logger.Warn("i18n initialization failed, continuing with fallback keys", slog.Any("error", err))
	}

	chatRegistry := chat.NewPlaceholderRegistry()
	adapterRegistry := channel.NewAdapterRegistry()
	pluginManager := plugin.NewManager()

	m := metrics.New()

	wasmCtx := context.Background()

	rt, err := wasmrt.NewRuntime(wasmCtx, wasmrt.Config{
		CacheDir: cfg.Admin.ModulesDir + "/.cache",
	})
	if err != nil {
		logger.Error("failed to create wasm runtime", slog.Any("error", err))
		os.Exit(1)
	}

	rt.SetMetrics(m)

	hostAPI := hostapi.NewHostAPI(hostapi.Dependencies{})
	hostAPI.SetMetrics(m)

	if err := hostAPI.RegisterHostModule(wasmCtx, rt); err != nil {
		logger.Error("failed to register wasm host module", slog.Any("error", err))
		os.Exit(1)
	}

	var senderAPI *plugin.SenderAPI
	wasmSendFunc := adapter.SendFunc(func(ctx context.Context, channelType model.ChannelType, chatID string, text string) error {
		if senderAPI == nil {
			return fmt.Errorf("sender API not initialized")
		}
		msg := model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: text, Style: model.StylePlain},
			},
		}
		return senderAPI.ReplyToChat(ctx, channelType, chatID, msg)
	})

	wasmLoader := adapter.NewLoader(rt, hostAPI, wasmSendFunc)
	wasmLoader.SetMetrics(m)

	triggerRegistry := trigger.NewRegistry()
	wasmLoader.SetTriggerRegistry(triggerRegistry)

	triggerRouter := trigger.NewRouter(triggerRegistry, pluginManager)
	cronScheduler := trigger.NewCronScheduler(triggerRouter)
	triggerRegistry.SetCronScheduler(cronScheduler)

	var blobStore adminapi.BlobStore
	switch cfg.Admin.BlobStore {
	case "s3":
		s3Store, s3Err := adminapi.NewS3BlobStore(wasmCtx, adminapi.S3BlobStoreConfig{
			Bucket:    cfg.Admin.S3.Bucket,
			Region:    cfg.Admin.S3.Region,
			Endpoint:  cfg.Admin.S3.Endpoint,
			AccessKey: cfg.Admin.S3.AccessKey,
			SecretKey: cfg.Admin.S3.SecretKey,
			Prefix:    cfg.Admin.S3.Prefix,
		})
		if s3Err != nil {
			logger.Error("failed to create S3 blob store", slog.Any("error", s3Err))
			os.Exit(1)
		}
		blobStore = s3Store
		logger.Info("using S3 blob store", slog.String("bucket", cfg.Admin.S3.Bucket))
	default:
		fsStore, fsErr := adminapi.NewLocalFSBlobStore(cfg.Admin.ModulesDir)
		if fsErr != nil {
			logger.Error("failed to create blob store", slog.Any("error", fsErr))
			os.Exit(1)
		}
		blobStore = fsStore
		logger.Info("using local filesystem blob store", slog.String("dir", cfg.Admin.ModulesDir))
	}

	var redisClient *redis.Client
	if cfg.Redis.Addr != "" {
		rc, redisErr := database.NewRedisClient(wasmCtx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		if redisErr != nil {
			logger.Warn("Redis not available, Redis dialog storage disabled", slog.Any("error", redisErr))
		} else {
			redisClient = rc
			cronScheduler.SetRedis(rc)
			logger.Info("connected to Redis", slog.String("addr", cfg.Redis.Addr))
		}
	}

	var userRepo user.UserRepository
	var accountRepo user.AccountRepository
	var roleStore role.Store
	var pluginStore adminapi.PluginStore
	var versionStore adminapi.VersionStore
	var cmdPermStore adminapi.CommandPermStore
	var adminChatStore adminapi.AdminChatStore
	var authzStore authz.Store
	var universityProvider *providers.UniversityProvider
	var adminBus *pubsub.Bus

	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)
	if cfg.Database.Host != "" && cfg.Database.DBName != "" {
		pool, dbErr := database.NewPool(wasmCtx, connString)
		if dbErr != nil {
			logger.Warn("PostgreSQL not available, falling back to in-memory stores", slog.Any("error", dbErr))
		} else {
			if migErr := database.RunMigrations(connString); migErr != nil {
				logger.Warn("failed to run migrations, falling back to in-memory stores", slog.Any("error", migErr))
				pool.Close()
				pool = nil
			}
			if pool != nil {
				userRepo = user.NewPgUserRepo(pool)
				accountRepo = user.NewPgAccountRepo(pool)
				roleStore = role.NewPgStore(pool)
				pluginStore = adminapi.NewPgPluginStore(pool)
				versionStore = adminapi.NewPgVersionStore(pool)
				cmdPermStore = adminapi.NewPgCommandPermStore(pool)
				adminChatStore = adminapi.NewPgAdminChatStore(pool)
				authzStore = authz.NewPgStore(pool)
				universityProvider = providers.NewUniversityProvider(pool)
				adminBus = pubsub.NewBus(pool, connString, generateInstanceID())
				logger.Info("using PostgreSQL stores")
			}
		}
	}

	if userRepo == nil {
		userRepo = user.NewPlaceholderUserRepo()
		accountRepo = user.NewPlaceholderAccountRepo()
		logger.Warn("using in-memory user/account stores (data will be lost on restart)")
	}
	if roleStore == nil {
		roleStore = role.NewPlaceholderStore()
		logger.Warn("using in-memory role store (data will be lost on restart)")
	}
	if authzStore == nil {
		authzStore = authz.NewPlaceholderStore()
	}
	if pluginStore == nil {
		fileStore, fsErr := adminapi.NewFilePluginStore(cfg.Admin.ModulesDir)
		if fsErr != nil {
			logger.Error("failed to create file plugin store", slog.Any("error", fsErr))
			os.Exit(1)
		}
		pluginStore = fileStore
		logger.Info("using file-based plugin store")
	}
	if versionStore == nil {
		fileVerStore, fsErr := adminapi.NewFileVersionStore(cfg.Admin.ModulesDir)
		if fsErr != nil {
			logger.Error("failed to create file version store", slog.Any("error", fsErr))
			os.Exit(1)
		}
		versionStore = fileVerStore
		logger.Info("using file-based version store")
	}

	userService := user.NewService(userRepo, accountRepo)
	roleManager := role.NewManager(roleStore, logger)

	var authzProviders []authz.AttributeProvider
	var schemaContributors []authz.SchemaContributor
	if universityProvider != nil {
		authzProviders = append(authzProviders, universityProvider)
		schemaContributors = append(schemaContributors, universityProvider)
	}

	authorizer := authz.NewAuthorizer(authzStore, logger, authzProviders...)
	schemaBuilder := authz.NewRuleSchemaBuilder(authzStore, schemaContributors...)
	ruleSchemaHandler := adminapi.NewRuleSchemaHandler(schemaBuilder)

	if err := adminapi.AutoloadPlugins(wasmCtx, pluginStore, blobStore, wasmLoader, pluginManager); err != nil {
		logger.Warn("wasm autoload failed", slog.Any("error", err))
	}

	adapter.RegisterWasmPlugins(pluginManager, wasmLoader)

	var dialogStore storage.DialogStorage
	if redisClient != nil {
		dialogStore = storage.NewRedisStorage(redisClient)
		logger.Info("using Redis dialog storage")
	} else {
		dialogStore = &inMemoryDialogStorage{}
		logger.Warn("using in-memory dialog storage (state will be lost on restart)")
	}
	stateMgr := state.NewManager(dialogStore)

	adminHandler := adminapi.NewAdminHandler(
		pluginStore,
		blobStore,
		wasmLoader,
		pluginManager,
		rt,
		hostAPI,
		stateMgr,
		cmdPermStore,
		versionStore,
		cfg.Admin.APIKey,
		adminBus,
	)

	adminMux := http.NewServeMux()
	adminHandler.RegisterRoutes(adminMux)
	cmdPermHandler := adminapi.NewCommandPermHandler(cmdPermStore)
	cmdPermHandler.RegisterRoutes(adminMux)
	pluginPermHandler := adminapi.NewPluginPermHandler(pluginStore, wasmLoader, hostAPI, adminBus)
	pluginPermHandler.RegisterRoutes(adminMux)
	chatHandler := adminapi.NewChatHandler(adminChatStore, adapterRegistry)
	chatHandler.RegisterRoutes(adminMux)
	ruleSchemaHandler.RegisterRoutes(adminMux)
	httpTrigger := trigger.NewHTTPTriggerHandler(triggerRouter, triggerRegistry)
	httpTrigger.SetMetrics(m)
	adminMux.Handle("/api/triggers/http/", httpTrigger)

	adminMux.Handle("GET /metrics", promhttp.Handler())

	admin.RegisterStaticRoutes(adminMux)

	adminServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Admin.Port),
		Handler:      adminMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("starting Admin API HTTP server", slog.Int("port", cfg.Admin.Port))
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("admin API server error", slog.Any("error", err))
		}
	}()

	senderAPI = plugin.NewSenderAPI(adapterRegistry, userService, chatRegistry)

	projectStore := &placeholderProjectStore{}
	chatStore := &placeholderChatStore{}
	accountLinker := user.NewAccountLinker(accountRepo)

	allPlugins := []plugin.Plugin{
		broadcast.New(senderAPI, projectStore),
		pluginLink.New(senderAPI, accountLinker),
		project.New(senderAPI, projectStore, chatStore),
		resume.New(senderAPI, stateMgr),
		settings.New(senderAPI, userService),
	}

	for _, p := range allPlugins {
		for _, def := range p.Commands() {
			stateMgr.RegisterCommand(def)
		}
	}

	for _, wp := range pluginManager.All() {
		for _, def := range wp.Commands() {
			stateMgr.RegisterCommand(def)
		}
	}

	stateAdapter := channel.NewStateManagerAdapter(stateMgr)

	pluginManager.Load(allPlugins)

	cronScheduler.Start()

	updateRouter := plugin.NewUpdateRouter(pluginManager)

	channelMgr := channel.NewChannelManager(
		userService,
		updateRouter,
		stateAdapter,
		pluginManager,
		authorizer,
		adapterRegistry,
		logger,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if adminBus != nil {
		pluginFetcher := func(fCtx context.Context, id string) (*pubsub.PluginData, error) {
			rec, err := pluginStore.GetPlugin(fCtx, id)
			if err != nil {
				return nil, err
			}
			return &pubsub.PluginData{
				WasmKey:     rec.WasmKey,
				ConfigJSON:  rec.ConfigJSON,
				Permissions: rec.Permissions,
			}, nil
		}
		eventHandler := pubsub.NewAdminEventHandler(
			pluginFetcher, blobStore.Get, wasmLoader, pluginManager, hostAPI, stateMgr,
		)
		go func() {
			if err := adminBus.Subscribe(ctx, eventHandler.Handle); err != nil {
				logger.Error("pubsub subscriber stopped", slog.Any("error", err))
			}
		}()
		logger.Info("pub/sub subscriber started", slog.String("instance", adminBus.InstanceID()))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var commandNames []string
	for _, p := range pluginManager.All() {
		for _, def := range p.Commands() {
			commandNames = append(commandNames, def.Name)
		}
	}

	if cfg.Telegram.Token != "" {
		logger.Info("starting Telegram bot")
		tgBot, err := telegram.NewBot(cfg.Telegram.Token, channelMgr, logger)
		if err != nil {
			logger.Error("failed to create Telegram bot", slog.Any("error", err))
		} else {
			tgBot.RegisterCommands(commandNames)
			channelMgr.RegisterAdapter(tgBot.Adapter())
			go func() {
				if err := tgBot.Start(ctx); err != nil {
					logger.Error("Telegram bot stopped with error", slog.Any("error", err))
				}
			}()
		}
	} else {
		logger.Warn("Telegram token not configured, Telegram bot will not start")
	}

	if cfg.Discord.Token != "" {
		logger.Info("starting Discord bot")
		dcBot, err := discord.NewBot(cfg.Discord.Token, channelMgr, logger)
		if err != nil {
			logger.Error("failed to create Discord bot", slog.Any("error", err))
		} else {
			channelMgr.RegisterAdapter(dcBot.Adapter())
			go func() {
				if err := dcBot.Start(ctx); err != nil {
					logger.Error("Discord bot stopped with error", slog.Any("error", err))
				}
			}()
		}
	} else {
		logger.Warn("Discord token not configured, Discord bot will not start")
	}

	logger.Info("SuperBotGo started, waiting for shutdown signal")

	<-sigCh
	logger.Info("shutdown signal received, stopping...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("admin API server shutdown error", slog.Any("error", err))
	} else {
		logger.Info("admin API server stopped")
	}

	cronScheduler.Stop()

	if err := wasmLoader.Close(shutdownCtx); err != nil {
		logger.Error("wasm loader close error", slog.Any("error", err))
	}
	if err := rt.Close(shutdownCtx); err != nil {
		logger.Error("wasm runtime close error", slog.Any("error", err))
	}

	logger.Info("SuperBotGo stopped")

	_ = roleManager
}

func generateInstanceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
