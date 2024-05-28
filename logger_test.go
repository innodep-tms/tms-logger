package tmslogger

import (
	"strconv"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestInitUseOnlyLogService(t *testing.T) {
	InitLogger(TmsLoggerConfig{
		UseNtmsLogServiceSink: true,
		NtmsLogServiceHostStr: "http://localhost:8080/ntms-log-service/api/v1/log/data",
		ServiceName:           "TmsLogger-Package-InitTest"})

	for i := 0; i < 1000; i++ {
		zap.S().Debug("Debug")
		zap.S().Info("Info")
		zap.S().Warn("Warn")
		zap.S().Error("Error")
	}
	zap.S().Sync()
	time.Sleep(time.Second)
	if lc == nil {
		t.Error("Init Fail : LogClient is nil")
	}
}

func TestInitUseOnlyConsoleOutput(t *testing.T) {
	InitLogger(TmsLoggerConfig{
		UseStdoutSink: true})

	for i := 0; i < 1000; i++ {
		zap.S().Debug("Debug")
		zap.S().Info("Info")
		zap.S().Warn("Warn")
		zap.S().Error("Error")
	}
	zap.S().Sync()
	time.Sleep(time.Second)
	if lc != nil {
		t.Error("error logclient must be nil")
	}
}

func TestInitUseOnlyLumberjackLogger(t *testing.T) {
	InitLogger(TmsLoggerConfig{
		UseLumberjackSink: true})

	for i := 0; i < 1000; i++ {
		zap.S().Debug("Debug")
		zap.S().Info("Info")
		zap.S().Warn("Warn")
		zap.S().Error("Error")
	}
	zap.S().Sync()
	time.Sleep(time.Second)
	PrintLogClientStatic()
}

func TestInitUseConsoleAndLumberjackLogger(t *testing.T) {
	InitLogger(TmsLoggerConfig{
		UseStdoutSink:     true,
		UseLumberjackSink: true})

	for i := 0; i < 1000; i++ {
		zap.S().Debug("Debug")
		zap.S().Info("Info")
		zap.S().Warn("Warn")
		zap.S().Error("Error")
	}
	zap.S().Sync()
	time.Sleep(time.Hour)
	PrintLogClientStatic()
}

func TestPanic(t *testing.T) {
	var err error
	InitLogger(TmsLoggerConfig{
		UseNtmsLogServiceSink: true,
		NtmsLogServiceHostStr: "http://localhost:8080/ntms-log-service/api/v1/log/data",
		ServiceName:           "TmsLogger-Package-InitTest"})

	for i := 0; i < 1000; i++ {
		zap.S().Debug("Debug" + strconv.Itoa(i))
		zap.S().Info("Info" + strconv.Itoa(i))
		zap.S().Warn("Warn" + strconv.Itoa(i))
		zap.S().Error("Error" + strconv.Itoa(i))
		if i == 300 {
			err.Error()
		}
	}
}
