package config

import (
	"os"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server    HTTPConfig          `yaml:"server"`
	Github    githubapp.Config    `yaml:"github"`
	AppConfig MyApplicationConfig `yaml:"app_configuration"`
}

type HTTPConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

type MyApplicationConfig struct {
	RedisHostPort   string `yaml:"redis_hostport"`
	WebhookProxyURL string `yaml:"webhook_proxy_url"`
	RequiredLabel   string `yaml:"required_label,omitempty"`
	BotUsername     string `yaml:"bot_username,omitempty"`
}

func ReadConfig(path string) (*Config, error) {
	var c Config

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading server config file: %s", path)
	}

	if err := yaml.UnmarshalStrict(bytes, &c); err != nil {
		return nil, errors.Wrap(err, "failed parsing configuration file")
	}

	return &c, nil
}

func (c *Config) GetBotUsername() string {
	if c.AppConfig.BotUsername == "" {
		return "@instruct-lab-bot"
	}
	return c.AppConfig.BotUsername
}
