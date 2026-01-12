package server

import (
	"os"

	"github.com/xucx/llmapi"
	"github.com/xucx/llmapi/log"

	"go.yaml.in/yaml/v3"
)

var (
	// default server config
	C Config = Config{
		Log: log.ZapLoggerConfig{
			Level: "info",
		},
		Host: "0.0.0.0:9000",
	}
)

type Config struct {
	Log    log.ZapLoggerConfig `yaml:"log"`
	Host   string              `yaml:"host"`
	Tokens []string            `yaml:"tokens"`
	LLM    llmapi.Config       `yaml:"llm"`
}

func LoadConfig(f string) error {
	fd, err := os.ReadFile(f)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(fd, &C); err != nil {
		return err
	}

	return nil
}
