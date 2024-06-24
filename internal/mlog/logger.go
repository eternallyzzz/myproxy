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
	"time"
)

var (
	lock    sync.Mutex
	logger  *zap.Logger
	preFile *os.File
	levels  = map[string]zapcore.Level{
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

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0777)
		if err != nil {
			log.Println(err)
			return
		}

		fL := defaultLevel
		cL := defaultLevel

		if c != nil && c.FileLevel != "" {
			fL = levels[c.FileLevel]
		}
		if c != nil && c.ConsoleLevel != "" {
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

		logger = zap.New(zapcore.NewTee(consoleCore, fileCore),
			zap.AddCaller(),
			zap.AddCallerSkip(1),
			zap.AddStacktrace(zapcore.ErrorLevel),
		)

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
	logger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}

func Unwrap(err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
		logger.Error("", fields...)
	}
}

func UnwrapFatal(err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
		logger.Fatal("", fields...)
	}
}

func UnwrapWithMessage(msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
		logger.Error("", fields...)
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
