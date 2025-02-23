# tms-logger
## example
전역 zap logger 객체 생성에 필요한 config의 정보는 다음과 같다.    
```
// SkipDefaultConsoleOutput : if true, skip default console output (default : false)
// SkipDefaultLumberjackLogger : if true, skip default lumberjack logger (default : false)
// SkipDefaultNtmsLogService : if true, skip ntms-log-service use (default : false)
// LogLevel : log level (DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL / default : INFO) 
// LogPath : log file path (default : ./logs)
// LogFileName : log file name (default : app.log)
// LogMaxSize : log file max size (MB) (default : 30MB)
// LogMaxBackups : log file max backups(default : 10)
// LogMaxAge : log file max age days (default : 10) 
// LogCompress : log file compress (true, false / default : true)
// NtmsLogServiceHostStr : ntms-log-service host string.(default : http://ntms-log-service:8080/ntms-log-service/api/v1/log/data)
// ServiceVersion : service version
// ServiceName : service name. if not provide, can't use ntms-log-service
type TmsLoggerConfig struct {
	SkipDefaultConsoleOutput    bool
	SkipDefaultLumberjackLogger bool
	SkipDefaultNtmsLogService   bool
	LogLevel                    string
	LogPath                     string
	LogFileName                 string
	LogMaxSize                  int
	LogMaxBackups               int
	LogMaxAge                   int
	LogCompress                 *bool
	NtmsLogServiceHostStr       string
	ServiceVersion              string
	ServiceName                 string
	NtmsLogServiceChanSize      int
	NtmsLogServiceSendBufSize   int
}
```  
위의 컨피그 정보를 이용하여 전역 zap.Logger객체를 생성하는 예  
```
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

```
