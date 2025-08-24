package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	MEXC       MEXCConfig       `mapstructure:"mexc"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

type TelegramConfig struct {
	BotToken string `mapstructure:"bot_token"`
}

type MEXCConfig struct {
	WebSocketURL string `mapstructure:"websocket_url"`
}

type MonitoringConfig struct {
	TimeInterval int     `mapstructure:"time_interval"`
	PriceChange  float64 `mapstructure:"price_change"`
	MinVolume    int     `mapstructure:"min_volume"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/opt/mexc-monitor")
	viper.AddConfigPath("/etc/mexc-monitor")

	viper.SetDefault("mexc.websocket_url", "wss://wbs.mexc.com/ws")
	viper.SetDefault("monitoring.time_interval", 5)
	viper.SetDefault("monitoring.price_change", 2.0)
	viper.SetDefault("monitoring.min_volume", 5000)
	viper.SetDefault("database.path", "data/monitor.db")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.file", "logs/monitor.log")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			viper.WriteConfigAs("config.yaml")
		} else {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
