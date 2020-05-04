package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/tokenized/relationship-example/internal/node"
	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/db"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/rpcnode"
	"github.com/tokenized/smart-contract/pkg/spynode"
	"github.com/tokenized/smart-contract/pkg/spynode/handlers/data"
	"github.com/tokenized/smart-contract/pkg/storage"
	"github.com/tokenized/smart-contract/pkg/txbuilder"
)

var (
	buildVersion = "unknown"
	buildDate    = "unknown"
	buildUser    = "unknown"
)

func main() {

	// -------------------------------------------------------------------------
	// Logging

	logConfig := logger.NewDevelopmentConfig()
	logConfig.Main.Format |= logger.IncludeSystem | logger.IncludeMicro
	logConfig.Main.MinLevel = logger.LevelVerbose
	logConfig.EnableSubSystem(rpcnode.SubSystem)
	logConfig.EnableSubSystem(txbuilder.SubSystem)

	if strings.ToUpper(os.Getenv("LOG_FORMAT")) == "TEXT" {
		logConfig.IsText = true
	}

	logPath := os.Getenv("LOG_FILE_PATH")
	if len(logPath) > 0 {
		os.MkdirAll(path.Dir(os.Getenv("LOG_FILE_PATH")), os.ModePerm)
		logFileName := filepath.FromSlash(os.Getenv("LOG_FILE_PATH"))
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			panic(fmt.Sprintf("Failed to open log file : %v\n", err))
		}
		defer logFile.Close()

		logConfig.Main.AddWriter(logFile)
	}

	// Configure spynode logs
	logPath = os.Getenv("SPYNODE_LOG_FILE_PATH")
	if len(logPath) > 0 {
		spynodeConfig := logger.NewDevelopmentSystemConfig()
		spynodeConfig.SetFile(logPath)
		spynodeConfig.MinLevel = logger.LevelVerbose
		logConfig.SubSystems[spynode.SubSystem] = spynodeConfig
	} else {
		logConfig.EnableSubSystem(spynode.SubSystem)
	}

	ctx := logger.ContextWithLogConfig(context.Background(), logConfig)

	logger.Info(ctx, "Started : Application Initializing")
	defer logger.Info(ctx, "Completed")

	// -------------------------------------------------------------------------
	// Configuration

	cfg, err := config.Environment()
	if err != nil {
		logger.Fatal(ctx, "Failed to fetch config : %s", err)
	}

	config, err := cfg.Config()
	if err != nil {
		logger.Fatal(ctx, "Failed to convert config : %s", err)
	}

	// Mask sensitive values
	cfgSafe := cfg.SafeConfig()
	cfgJSON, err := json.MarshalIndent(cfgSafe, "", "    ")
	if err != nil {
		log.Fatalf("Marshalling Config to JSON : %v", err)
	}
	logger.Info(ctx, "Config : %s", string(cfgJSON))

	// -------------------------------------------------------------------------
	// Data

	masterDB, err := db.New(&db.StorageConfig{
		Bucket:     cfg.Storage.Bucket,
		Root:       cfg.Storage.Root,
		MaxRetries: cfg.AWS.MaxRetries,
		RetryDelay: cfg.AWS.RetryDelay,
	})
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize storage : %s", err)
	}

	// -------------------------------------------------------------------------
	// SPY Node

	spyStorageConfig := storage.NewConfig(cfg.NodeStorage.Bucket, cfg.NodeStorage.Root)
	spyStorageConfig.SetupRetry(cfg.AWS.MaxRetries, cfg.AWS.RetryDelay)

	var spyStorage storage.Storage
	if strings.ToLower(spyStorageConfig.Bucket) == "standalone" {
		spyStorage = storage.NewFilesystemStorage(spyStorageConfig)
	} else {
		spyStorage = storage.NewS3Storage(spyStorageConfig)
	}

	spyConfig, err := data.NewConfig(config.Net, cfg.SpyNode.Address, cfg.SpyNode.UserAgent,
		cfg.SpyNode.StartHash, cfg.SpyNode.UntrustedNodes, cfg.SpyNode.SafeTxDelay,
		cfg.SpyNode.ShotgunCount)
	if err != nil {
		logger.Fatal(ctx, "Failed to create spynode config : %s", err)
	}

	spyNode := spynode.NewNode(spyConfig, spyStorage)

	spyNode.SetupRetry(cfg.SpyNode.MaxRetries, cfg.SpyNode.RetryDelay)

	// -------------------------------------------------------------------------
	// RPC Node

	rpcConfig := &rpcnode.Config{
		Host:       cfg.RpcNode.Host,
		Username:   cfg.RpcNode.Username,
		Password:   cfg.RpcNode.Password,
		MaxRetries: cfg.RpcNode.MaxRetries,
		RetryDelay: cfg.RpcNode.RetryDelay,
	}

	rpcNode, err := rpcnode.NewNode(rpcConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to create rpc node : %s", err)
	}

	// -------------------------------------------------------------------------
	// Wallet

	wallet, err := wallet.NewWallet(config, cfg.Key)
	if err != nil {
		logger.Fatal(ctx, "Failed to create wallet : %s", err)
	}

	// -------------------------------------------------------------------------
	// Node

	node, err := node.NewNode(config, masterDB, wallet, rpcNode, spyNode)
	if err != nil {
		logger.Fatal(ctx, "Failed to create node : %s", err)
	}

	wg := sync.WaitGroup{}

	if err := node.Load(ctx); err != nil {
		logger.Fatal(ctx, "Failed to load node : %s", err)
	}

	// Make a channel to listen for errors coming from the spynode. Use a buffered channel so the
	//   goroutine can exit if we don't collect this error.
	nodeErrors := make(chan error, 1)

	// Start the spynode.
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(ctx, "Node Running")
		nodeErrors <- node.Run(ctx)
	}()

	// -------------------------------------------------------------------------
	// Start SpyNode

	// Make a channel to listen for errors coming from the spynode. Use a buffered channel so the
	//   goroutine can exit if we don't collect this error.
	spyNodeErrors := make(chan error, 1)

	// Start the spynode.
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(ctx, "SpyNode Running")
		spyNodeErrors <- spyNode.Run(ctx)
	}()

	// -------------------------------------------------------------------------
	// Setup shutdown from system signals

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)

	// -------------------------------------------------------------------------
	// Stop daemon on error or signal

	// Blocking main and waiting for shutdown.
	select {
	case err := <-nodeErrors:
		if err != nil {
			logger.Error(ctx, "Error running node: %v", err)
		}

		// Asking teller to shutdown.
		if err := spyNode.Stop(ctx); err != nil {
			logger.Fatal(ctx, "Could not stop node: %v", err)
		}

	case err := <-spyNodeErrors:
		if err != nil {
			logger.Error(ctx, "Error running spynode: %v", err)
		}

		// Asking teller to shutdown.
		if err := node.Stop(ctx); err != nil {
			logger.Fatal(ctx, "Could not stop teller: %v", err)
		}

	case <-osSignals:
		logger.Info(ctx, "Start shutdown...")

		// Asking spynode to shutdown.
		if err := spyNode.Stop(ctx); err != nil {
			logger.Fatal(ctx, "Could not stop spynode: %v", err)
		}

		// Asking teller to shutdown.
		if err := node.Stop(ctx); err != nil {
			logger.Fatal(ctx, "Could not stop teller: %v", err)
		}
	}

	wg.Wait()
}
