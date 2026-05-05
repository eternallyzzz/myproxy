package mlog

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"myproxy/pkg/models"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	lock     sync.Mutex
	logger   atomic.Pointer[zap.Logger]
	preFile  *os.File
	levels   = map[string]zapcore.Level{
		"debug": zap.DebugLevel,
		"info":  zap.InfoLevel,
		"warn":  zap.WarnLevel,
		"error": zap.ErrorLevel,
		"fatal": zap.FatalLevel,
	}
	defaultLevel = levels["warn"]
	workDir, _   = os.Getwd()
)

func Init(c *models.Log) error {
	if c == nil {
		c = &models.Log{}
	}
	f := func() {
		developmentEncoderConfig := zap.NewDevelopmentEncoderConfig()
		developmentEncoderConfig.StacktraceKey = ""
		developmentEncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		developmentEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(developmentEncoderConfig)

		fileEncoderConfig := zap.NewProductionEncoderConfig()
		fileEncoderConfig.StacktraceKey = ""
		fileEncoderConfig.EncodeCaller = nil
		fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		fileEncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
		fileEncoder := zapcore.NewConsoleEncoder(fileEncoderConfig)

		logPath := fmt.Sprintf("%s/error_%s.log", workDir, time.Now().Format(time.DateOnly))
		if c.LogFilePath != "" {
			logPath = fmt.Sprintf("%s/error_%s.log", c.LogFilePath, time.Now().Format(time.DateOnly))
		}

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
		if err != nil {
			log.Println(err)
			return
		}

		fL := defaultLevel
		cL := defaultLevel

		if c.FileLevel != "" {
			fL = levels[c.FileLevel]
		}
		if c.ConsoleLevel != "" {
			cL = levels[c.ConsoleLevel]
		}

		fileCore := zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(file),
			fL,
		)

		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			cL,
		)

		lock.Lock()
		defer lock.Unlock()

		l := zap.New(zapcore.NewTee(consoleCore, fileCore),
			zap.AddCaller(),
			zap.AddCallerSkip(1),
			zap.AddStacktrace(zapcore.ErrorLevel),
		)
		logger.Store(l)

		if preFile != nil {
			_ = preFile.Close()
		}

		preFile = file
	}

	f()

	go func() {
		for {
			now := time.Now()
			nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			time.Sleep(time.Until(nextDay))

			f()
		}
	}()
	return nil
}

func Info(msg string, fields ...zap.Field) {
	logger.Load().Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Load().Error(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Load().Warn(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	logger.Load().Debug(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.Load().Fatal(msg, fields...)
}

func Unwrap(err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
		logger.Load().Error("", fields...)
	}
}

func UnwrapFatal(err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
		logger.Load().Fatal("", fields...)
	}
}

func UnwrapWithMessage(msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
		logger.Load().Error(msg, fields...)
	}
}

func Ignore(err error) bool {
	if strings.Contains(err.Error(), closeStreamErr) || strings.Contains(err.Error(), opErr) ||
		strings.Contains(err.Error(), noPeerResp) || strings.Contains(err.Error(), finalSizeErr) {
		return true
	}
	return false
}

var (
	closeStreamErr = "read from closed stream"
	opErr          = "use of closed network connection"
	noPeerResp     = "peer did not respond to CONNECTION_CLOSE"
	finalSizeErr   = "end of stream occurs before prior data"
)
