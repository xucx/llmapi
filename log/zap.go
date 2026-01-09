package log

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type ZapLoggerConfig struct {
	File   string `yaml:"file"`
	Format string `yaml:"format"`
	Level  string `yaml:"level"`
}

type ZapLogger struct {
	s *zap.SugaredLogger
}

func (z *ZapLogger) Log(level Level, args ...any) {
	z.s.Log(zapcore.Level(level), args...)
}

func (z *ZapLogger) Logf(level Level, format string, args ...any) {
	z.s.Logf(zapcore.Level(level), format, args...)
}

func (z *ZapLogger) Logw(level Level, msg string, kvs ...any) {
	z.s.Logw(zapcore.Level(level), msg, kvs...)
}

func WarpZapLogger(l *zap.Logger, name ...string) Logger {
	if len(name) > 0 {
		l = l.Named(strings.Join(name, "."))
	}
	return &ZapLogger{
		s: l.Sugar(),
	}
}

func NewZapLogger(conf ZapLoggerConfig) *zap.Logger {
	logLevel, _ := LevelFromString(conf.Level)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var (
		syncer  zapcore.WriteSyncer
		encoder zapcore.Encoder
	)

	if conf.File != "" {
		syncer = NoSync(&lumberjack.Logger{
			Filename:   conf.File, // Location of the log file
			MaxSize:    10,        // Maximum file size (in MB)
			MaxBackups: 3,         // Maximum number of old files to retain
			MaxAge:     28,        // Maximum number of days to retain old files
			Compress:   true,      // Whether to compress/archive old files
			LocalTime:  true,      // Use local time for timestamps
		})
	} else {
		syncer = os.Stderr
	}

	if conf.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	writer := zapcore.Lock(syncer)

	return zap.New(
		zapcore.NewCore(
			encoder,
			writer,
			logLevel,
		),
	)

}

func LevelFromString(levelStr string) (zapcore.Level, error) {
	defaultLevel := zapcore.InfoLevel
	level, err := zapcore.ParseLevel(levelStr)
	if err != nil {
		return defaultLevel, fmt.Errorf("log level %w parse fail", err)
	}

	switch level {
	case zap.DebugLevel, zap.InfoLevel, zap.WarnLevel, zap.ErrorLevel:
	default:
		return defaultLevel, fmt.Errorf("unsupported log level %s", level)
	}

	return level, nil
}

type noSync struct {
	io.Writer
}

func (noSync) Sync() error {
	return nil
}

func NoSync(w io.Writer) zapcore.WriteSyncer {
	return noSync{
		Writer: w,
	}
}
