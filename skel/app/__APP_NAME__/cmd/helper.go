package cmd

import "go.uber.org/zap"

func initDevLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}
