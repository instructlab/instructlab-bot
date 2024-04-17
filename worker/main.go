package main

import (
	"github.com/instructlab/instructlab-bot/worker/cmd"
	"go.uber.org/zap"
)

func main() {
	// Initialize global logger
	logLevel := zap.InfoLevel
	loggerConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(logLevel),
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, _ := loggerConfig.Build()
	defer func() {
		_ = logger.Sync()
	}()

	zap.ReplaceGlobals(logger)

	cmd.Execute()
}
