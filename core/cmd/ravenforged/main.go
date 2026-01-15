// Ravenforged is the main Ravenforge daemon.
package main

import (
	"fmt"
	"os"

	"github.com/ravenforge/ravenforge/core/internal/config"
	"github.com/ravenforge/ravenforge/core/internal/daemon"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	configFile string
	logLevel   string
	logFormat  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ravenforged",
		Short: "Ravenforge security platform daemon",
		Long: `Ravenforged is the core daemon for the Ravenforge cybersecurity platform.
It manages tool discovery, secure execution, job scheduling, and provides
the REST API for tool and pipeline execution.`,
		RunE: runDaemon,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "/etc/ravenforge/config.yaml", "Path to configuration file")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.Flags().StringVar(&logFormat, "log-format", "json", "Log format (json, console)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		return fmt.Errorf("initializing logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("starting ravenforged",
		zap.String("config", configFile),
	)

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		logger.Error("failed to load configuration", zap.Error(err))
		return err
	}

	// Override config with CLI flags if provided
	if cmd.Flags().Changed("log-level") {
		cfg.Logging.Level = logLevel
	}
	if cmd.Flags().Changed("log-format") {
		cfg.Logging.Format = logFormat
	}

	// Create and run daemon
	d, err := daemon.New(cfg, logger)
	if err != nil {
		logger.Error("failed to create daemon", zap.Error(err))
		return err
	}

	return d.Run()
}

func initLogger() (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		level = zapcore.InfoLevel
	}

	var config zap.Config
	if logFormat == "console" {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}
	config.Level = zap.NewAtomicLevelAt(level)

	return config.Build()
}
