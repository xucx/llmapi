package log

type Level int8

const (
	LevelDebug Level = iota - 1
	LevelInfo
	LevelWarn
	LevelError
	LevelDPanic
	LevelPanic
	LevelFatal

	_minLevel = LevelDebug
	_maxLevel = LevelFatal

	InvalidLevel = _maxLevel + 1
)

type Logger interface {
	Log(level Level, args ...any)
	Logf(level Level, format string, args ...any)
	Logw(level Level, msg string, kvs ...any)
}

var log Logger = &nopLogger{}

func Debug(args ...any)                  { log.Log(LevelDebug, args...) }
func Debugf(format string, args ...any)  { log.Logf(LevelDebug, format, args...) }
func Debugw(msg string, kvs ...any)      { log.Logw(LevelDebug, msg, kvs...) }
func Info(args ...any)                   { log.Log(LevelInfo, args...) }
func Infof(format string, args ...any)   { log.Logf(LevelInfo, format, args...) }
func Infow(msg string, kvs ...any)       { log.Logw(LevelInfo, msg, kvs...) }
func Warn(args ...any)                   { log.Log(LevelWarn, args...) }
func Warnf(format string, args ...any)   { log.Logf(LevelWarn, format, args...) }
func Warnw(msg string, kvs ...any)       { log.Logw(LevelWarn, msg, kvs...) }
func Error(args ...any)                  { log.Log(LevelError, args...) }
func Errorf(format string, args ...any)  { log.Logf(LevelError, format, args...) }
func Errorw(msg string, kvs ...any)      { log.Logw(LevelError, msg, kvs...) }
func DPanic(args ...any)                 { log.Log(LevelDPanic, args...) }
func DPanicf(format string, args ...any) { log.Logf(LevelDPanic, format, args...) }
func DPanicw(msg string, kvs ...any)     { log.Logw(LevelDPanic, msg, kvs...) }
func Panic(args ...any)                  { log.Log(LevelPanic, args...) }
func Panicf(format string, args ...any)  { log.Logf(LevelPanic, format, args...) }
func Panicw(msg string, kvs ...any)      { log.Logw(LevelPanic, msg, kvs...) }
func Fatal(args ...any)                  { log.Log(LevelFatal, args...) }
func Fatalf(format string, args ...any)  { log.Logf(LevelFatal, format, args...) }
func Fatalw(msg string, kvs ...any)      { log.Logw(LevelFatal, msg, kvs...) }

type nopLogger struct{}

func (*nopLogger) Log(level Level, args ...any)                 {}
func (*nopLogger) Logf(level Level, format string, args ...any) {}
func (*nopLogger) Logw(level Level, msg string, kvs ...any)     {}

func SetLogger(l Logger) {
	log = l
}
