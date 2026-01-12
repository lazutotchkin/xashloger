package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type LogsConfig struct {
	RetainDays int    `mapstructure:"retain-days"`
	LogDir     string `mapstructure:"log-dir"`
	Color      bool   `mapstructure:"color"`
}

type Database struct {
	Path     string `mapstructure:"path"`
	Dir      string `mapstructure:"dir"`
	Name     string `mapstructure:"name"`
	EventTTL int    `mapstructure:"event-ttl"`
}

type FlagsConfig struct {
	Production bool `mapstructure:"production"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
	Port int    `mapstructure:"port"`
}

type WebServerConfig struct {
	Addr       string     `mapstructure:"addr"`
	Port       int        `mapstructure:"port"`
	IpFirewall IpFirewall `mapstructure:"ip-firewall"`
}

type IpFirewall struct {
	Enabled bool     `mapstructure:"enabled"`
	Allow   []string `mapstructure:"allow"`
}

type RCON struct {
	Password string `mapstructure:"password"`
}

type MailConfig struct {
	SMTPHost string   `mapstructure:"smtp-host"`
	SMTPPort string   `mapstructure:"smtp-port"`
	From     string   `mapstructure:"from"`
	To       []string `mapstructure:"to"`
}

type TrackingServer struct {
	Name         string `mapstructure:"name"`
	Addr         string `mapstructure:"addr"`
	Port         int    `mapstructure:"port"`
	RCONPassword string `mapstructure:"rcon-password"`
	Enabled      bool   `mapstructure:"enabled"`
}

type Config struct {
	Server          ServerConfig     `mapstructure:"server"`
	WebServer       WebServerConfig  `mapstructure:"web-server"`
	Flags           FlagsConfig      `mapstructure:"flags"`
	Logs            LogsConfig       `mapstructure:"logs"`
	Database        Database         `mapstructure:"database"`
	RCON            RCON             `mapstructure:"rcon"`
	Mail            MailConfig       `mapstructure:"mail"`
	TrackingServers []TrackingServer `mapstructure:"tracking-servers"`
}

func Load(dev bool) (*Config, error) {

	var dir string
	if dev {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working dir: %w", err)
		}
		dir = filepath.Join(wd, "configs")
	} else {
		exePath, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}
		dir = filepath.Join(filepath.Dir(exePath), "configs")
	}
	viper.AddConfigPath(dir)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
