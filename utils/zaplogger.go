// ============================================
// 3. ZAP LOGGER (Structured Logging)
// ============================================
package utils

import (
	"github.com/madcok-co/unicorn"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLogger struct {
	logger *zap.SugaredLogger
}

func NewZapLogger(development bool) (*ZapLogger, error) {
	var config zap.Config

	if development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &ZapLogger{
		logger: logger.Sugar(),
	}, nil
}

func (l *ZapLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debugw(msg, keysAndValues...)
}

func (l *ZapLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Infow(msg, keysAndValues...)
}

func (l *ZapLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warnw(msg, keysAndValues...)
}

func (l *ZapLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Errorw(msg, keysAndValues...)
}

func (l *ZapLogger) With(keysAndValues ...interface{}) unicorn.Logger {
	return &ZapLogger{
		logger: l.logger.With(keysAndValues...),
	}
}

func (l *ZapLogger) Sync() error {
	return l.logger.Sync()
}
