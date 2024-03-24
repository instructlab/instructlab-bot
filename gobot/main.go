package main

import (
	"github.com/instruct-lab/instruct-lab-bot/gobot/bot"
	"go.uber.org/zap"
)

func main() {
	// Initlaize global logger
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

	logger.Sugar().Info("Starting bot...")
	err := bot.Run(logger)
	if err != nil {
		logger.Sugar().Errorf("Error running bot: %v", err)
	}
}
