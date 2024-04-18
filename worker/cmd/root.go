package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	RedisHost string
	Debug     bool
	TestMode  bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&RedisHost, "redis", "r", "localhost:6379", "The Redis instance to connect to")
	rootCmd.PersistentFlags().BoolVarP(&TestMode, "test", "t", false, "Enable test mode - do not run generate or post to S3")
	rootCmd.PersistentFlags().BoolVarP(&Debug, "debug", "d", false, "Enable debug logging")
}

var rootCmd = &cobra.Command{
	Use:   "worker",
	Short: "Worker receives jobs from a Redis queue and processes them.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig(cmd)
	},
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

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()
	v.SetConfigName("instructlab-worker")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/instructlab-worker")
	v.AddConfigPath("/etc/instructlab-worker")
	if err := v.ReadInConfig(); err != nil {
		// It's okay if there isn't a config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	v.SetEnvPrefix("ILWORKER")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	bindFlags(cmd, v)
	return nil
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		configName := f.Name
		if !f.Changed && v.IsSet(configName) {
			val := v.Get(configName)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
