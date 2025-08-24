package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"mexc-monitor/internal/config"
	"mexc-monitor/internal/database"
	"mexc-monitor/internal/monitor"
	"mexc-monitor/internal/telegram"

	log "github.com/sirupsen/logrus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	setupLogging(cfg)

	log.Info("Starting MEXC Monitor...")

	db, err := database.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	bot, err := telegram.NewBot(cfg.Telegram.BotToken, db)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	mon, err := monitor.New(cfg, db, bot)
	if err != nil {
		log.Fatalf("Failed to initialize monitor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := mon.Start(ctx); err != nil {
			log.Errorf("Monitor error: %v", err)
		}
	}()

	go func() {
		if err := bot.Start(); err != nil {
			log.Errorf("Telegram bot error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")
	cancel()
}

func setupLogging(cfg *config.Config) {
	level, err := log.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Warnf("Failed to create logs directory: %v", err)
		return
	}

	file, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Warnf("Failed to open log file: %v", err)
		return
	}

	log.SetOutput(file)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}
