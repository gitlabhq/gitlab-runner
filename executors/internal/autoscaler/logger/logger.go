package logger

import (
	"fmt"
	"io"
	"log"

	"github.com/hashicorp/go-hclog"
	"github.com/sirupsen/logrus"
)

type Logger struct {
	entry *logrus.Entry
	name  string
}

var (
	logrusToHclog = []hclog.Level{
		logrus.PanicLevel: hclog.Error,
		logrus.FatalLevel: hclog.Error,
		logrus.ErrorLevel: hclog.Error,
		logrus.WarnLevel:  hclog.Warn,
		logrus.InfoLevel:  hclog.Info,
		logrus.DebugLevel: hclog.Debug,
		logrus.TraceLevel: hclog.Trace,
	}

	hclogToLogrus = []logrus.Level{
		hclog.NoLevel: logrus.InfoLevel,
		hclog.Trace:   logrus.TraceLevel,
		hclog.Debug:   logrus.DebugLevel,
		hclog.Info:    logrus.InfoLevel,
		hclog.Warn:    logrus.WarnLevel,
		hclog.Error:   logrus.ErrorLevel,
		hclog.Off:     logrus.InfoLevel,
	}
)

func New(entry *logrus.Entry) *Logger {
	entry = entry.Dup()
	if entry.Logger == nil {
		entry.Logger = logrus.StandardLogger()
	}

	return &Logger{entry: entry}
}

func (l *Logger) level(lvl hclog.Level) logrus.Level {
	return hclogToLogrus[lvl]
}

func (l *Logger) fields(args []any) logrus.Fields {
	if len(args) == 0 {
		return nil
	}

	if len(args)%2 != 0 {
		args = append(args, "<unknown>")
	}

	fields := make(logrus.Fields, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", args[i])
		}
		fields[key] = args[i+1]
	}

	return fields
}

func (l *Logger) Log(level hclog.Level, msg string, args ...interface{}) {
	entry := l.entry
	if len(args) > 0 {
		entry = entry.WithFields(l.fields(args))
	}

	entry.Log(l.level(level), msg)
}

func (l *Logger) Trace(msg string, args ...interface{}) {
	l.Log(hclog.Trace, msg, args...)
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.Log(hclog.Debug, msg, args...)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.Log(hclog.Info, msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.Log(hclog.Warn, msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.Log(hclog.Error, msg, args...)
}

func (l *Logger) IsTrace() bool {
	return l.entry.Logger.IsLevelEnabled(logrus.TraceLevel)
}

func (l *Logger) IsDebug() bool {
	return l.entry.Logger.IsLevelEnabled(logrus.DebugLevel)
}

func (l *Logger) IsInfo() bool {
	return l.entry.Logger.IsLevelEnabled(logrus.InfoLevel)
}

func (l *Logger) IsWarn() bool {
	return l.entry.Logger.IsLevelEnabled(logrus.WarnLevel)
}

func (l *Logger) IsError() bool {
	return l.entry.Logger.IsLevelEnabled(logrus.ErrorLevel)
}

func (l *Logger) ImpliedArgs() []any {
	if len(l.entry.Data) == 0 {
		return nil
	}

	fields := make([]any, len(l.entry.Data)*2)
	for key, val := range l.entry.Data {
		fields = append(fields, key, val)
	}

	return fields
}

func (l *Logger) With(args ...interface{}) hclog.Logger {
	if len(args) == 0 {
		return l
	}

	return New(l.entry.WithFields(l.fields(args)))
}

func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) Named(name string) hclog.Logger {
	if l.name != "" {
		name = l.name + "." + name
	}

	return l.ResetNamed(name)
}

func (l *Logger) ResetNamed(name string) hclog.Logger {
	logger := New(l.entry.WithFields(logrus.Fields{"subsystem": name}))
	logger.name = name

	return logger
}

func (l *Logger) SetLevel(level hclog.Level) {
	l.entry.Logger.SetLevel(l.level(level))
}

func (l *Logger) GetLevel() hclog.Level {
	return logrusToHclog[l.entry.Logger.GetLevel()]
}

func (l *Logger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	logger := hclog.Default()
	logger.SetLevel(l.GetLevel())
	logger.Named(l.name)
	logger.With(l.ImpliedArgs()...)

	return logger.StandardLogger(opts)
}

func (l *Logger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	logger := hclog.Default()
	logger.SetLevel(l.GetLevel())
	logger.Named(l.name)
	logger.With(l.ImpliedArgs()...)

	return logger.StandardWriter(opts)
}
