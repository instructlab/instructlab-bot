package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	RedisHost string
	Debug     bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&RedisHost, "redis", "r", "localhost:6379", "The Redis instance to connect to")
	rootCmd.PersistentFlags().BoolVarP(&Debug, "debug", "d", false, "Enable debug logging")
}

var rootCmd = &cobra.Command{
	Use:   "worker",
	Short: "Worker receives jobs from a Redis queue and processes them.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initLogger(debug bool) *zap.Logger {
	level := zap.InfoLevel

	if debug {
		level = zap.DebugLevel
	}

	loggerConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, _ := loggerConfig.Build()
	return logger
}
