package tmslogger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	ps "github.com/mitchellh/go-ps"
	"github.com/mitchellh/panicwrap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var loglevel zap.AtomicLevel = zap.NewAtomicLevel()
var lc zap.Sink
var zapCoreMap map[string]zapcore.Core = make(map[string]zapcore.Core)

type lumberjackSink struct {
	*lumberjack.Logger
}

// UseZapStdoutSink : if true, use console output
// UseZapLumberjackSink : if true, use lumberjack logger
// UseNtmsLogServiceSink : if true, use ntms-log-service
// LogLevel : log level (DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL)
// LogPath : log file path
// LogFileName : log file name
// LogMaxSize : log file max size (MB)
// LogMaxBackups : log file max backups
// LogMaxAge : log file max age (days)
// LogCompress : log file compress (true, false)
// NtmsLogServiceHostStr : ntms-log-service host string. if not provide, can't use ntms-log-service
// ServiceVersion : service version
// ServiceName : service name. if not provide, can't use ntms-log-service
type TmsLoggerConfig struct {
	UseStdoutSink             bool
	UseLumberjackSink         bool
	UseNtmsLogServiceSink     bool
	LogLevel                  string
	LogPath                   string
	LogFileName               string
	LogMaxSize                int
	LogMaxBackups             int
	LogMaxAge                 int
	LogCompress               *bool
	NtmsLogServiceHostStr     string
	ServiceVersion            string
	ServiceName               string
	NtmsLogServiceChanSize    int
	NtmsLogServiceSendBufSize int
}

type PanicLog struct {
	RegDate     string `json:"T"`
	ServiceName string `json:"N"`
	LogLevel    string `json:"L"`
	Message     string `json:"M"`
	StackTrace  string `json:"S"`
}

func (lumberjackSink) Sync() error { return nil }

// Initialize logger
func InitLogger(opt TmsLoggerConfig) {
	SetGlobalZapLogger(opt)
	// debugging 시 child process가 생성될 경우 breakpoint가 무용지물이 되는것을 방지하기 위해 fork하지 않는다.
	// 이 경우 panic log또한 처리되지 않음.
	if !isDebuggingRightNow() {
		// panicwrap.BasicWrap() is used to catch panic when chile process exit.
		// so it suspends until the child process is terminated.
		exitStatus, _ := panicwrap.BasicWrap(PanicHandler)
		if exitStatus >= 0 {
			os.Exit(exitStatus)
		}
	}
}

func isDebuggingRightNow() bool {
	pid := os.Getpid()
	for pid != 0 {
		switch p, err := ps.FindProcess(pid); {
		case err != nil:
			return false
		case p == nil:
			pid = 0
			break
		case p.Executable() == "dlv":
			return true
		case p.Executable() == "dlv.exe":
			return true
		default:
			pid = p.PPid()
		}
	}
	return false
}

func SetGlobalZapLogger(opt TmsLoggerConfig) {
	var logger *zap.Logger
	var zapCores []zapcore.Core
	dec := zap.NewDevelopmentEncoderConfig()

	defaultLogCompress := true

	if opt.LogLevel == "" {
		opt.LogLevel = "INFO"
	}
	SetLogLevel(opt.LogLevel)

	// stdout에 로그를 출력하는 custom package가 없으면서 zap의 stdout sink를 사용하고자 하는 경우
	// stdout에 로그를 출력하는 zap.Core를 생성한다.
	if opt.UseStdoutSink {
		stdOutCore := makeStdOutCore(&dec)
		zapCores = append(zapCores, stdOutCore)
		zapCoreMap["stdout"] = stdOutCore
	}

	if opt.UseLumberjackSink {
		if opt.LogPath == "" {
			opt.LogPath = "./logs"
		}
		if opt.LogFileName == "" {
			opt.LogFileName = "app.log"
		}
		if opt.LogMaxSize == 0 {
			opt.LogMaxSize = 10
		}
		if opt.LogMaxBackups == 0 {
			opt.LogMaxBackups = 10
		}
		if opt.LogMaxAge == 0 {
			opt.LogMaxAge = 30
		}
		if opt.LogCompress == nil {
			opt.LogCompress = &defaultLogCompress
		}
		if _, err := os.Stat(opt.LogPath); os.IsNotExist(err) {
			os.Mkdir(opt.LogPath, os.ModePerm)
		}
		lumberjackCore := makeLumberjackCore(opt, &dec)
		zapCores = append(zapCores, lumberjackCore)
		zapCoreMap["lumberjack"] = lumberjackCore
	}

	if opt.UseNtmsLogServiceSink {
		if opt.NtmsLogServiceHostStr == "" {
			opt.NtmsLogServiceHostStr = "http://ntms-log-service:8080/ntms-log-service/api/v1/log/data"
		}
		if opt.ServiceName != "" {
			logClientCore := makeLogClientCore(opt, &dec)
			zapCores = append(zapCores, logClientCore)
			zapCoreMap["logClient"] = logClientCore
		}
	}
	core := zapcore.NewTee(zapCores...)

	logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	if opt.ServiceName != "" {
		logger = logger.Named(opt.ServiceName)
	}
	zap.ReplaceGlobals(logger)
}

func makeStdOutCore(dec *zapcore.EncoderConfig) zapcore.Core {
	consoleEncoder := zapcore.NewConsoleEncoder(*dec)
	stdOutWriter := zapcore.AddSync(os.Stdout)
	return zapcore.NewCore(consoleEncoder, stdOutWriter, loglevel)
}

func makeLumberjackCore(opt TmsLoggerConfig, dec *zapcore.EncoderConfig) zapcore.Core {
	lumberjackWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   opt.LogPath + "/" + opt.LogFileName,
		MaxSize:    opt.LogMaxSize, // megabytes
		MaxBackups: opt.LogMaxBackups,
		MaxAge:     opt.LogMaxAge,    // days
		Compress:   *opt.LogCompress, // disabled by default
	})
	consoleEncoder := zapcore.NewConsoleEncoder(*dec)
	return zapcore.NewCore(consoleEncoder, lumberjackWriter, loglevel)
}

func makeLogClientCore(opt TmsLoggerConfig, dec *zapcore.EncoderConfig) zapcore.Core {
	var err error
	if opt.ServiceName == "" {
		return nil
	}
	jsonEncoder := zapcore.NewJSONEncoder(*dec)
	lc, err = NewLogClient(
		opt.NtmsLogServiceHostStr,
		opt.ServiceVersion,
		opt.NtmsLogServiceChanSize,
		opt.NtmsLogServiceSendBufSize)
	if err != nil {
		return nil
	}
	logClientWriter := zapcore.AddSync(lc)
	return zapcore.NewCore(jsonEncoder, logClientWriter, loglevel)
}

func PrintLogClientStatic() {
	if lc != nil {
		failSetChannelCnt, failSendApiCnt := lc.(LogClientSink).GetStatic()
		fmt.Printf("channel overflow cnt : %d, request failed cnt : %d\n", failSetChannelCnt, failSendApiCnt)
	} else {
		fmt.Println("LogClient is not initialized.")
	}
}

func GetLogLevel() string {
	return loglevel.String()
}

func SetLogLevel(level string) {
	level = strings.ToUpper(level)
	switch level {
	case "DEBUG":
		// DebugLevel logs are typically voluminous, and are usually disabled in production.
		loglevel.SetLevel(zapcore.DebugLevel)
	case "INFO":
		// InfoLevel is the default logging priority.
		loglevel.SetLevel(zapcore.InfoLevel)
	case "WARN":
		// WarnLevel logs are more important than Info, but don't need individual human review.
		loglevel.SetLevel(zapcore.WarnLevel)
	case "ERROR":
		// ErrorLevel logs are high-priority. If an application is running smoothly, it shouldn't generate any error-level logs.
		loglevel.SetLevel(zapcore.ErrorLevel)
	case "DPANIC":
		// DPanicLevel logs are particularly important errors. In development the logger panics after writing the message.
		loglevel.SetLevel(zapcore.DPanicLevel)
	case "PANIC":
		// PanicLevel logs a message, then panics.
		loglevel.SetLevel(zapcore.PanicLevel)
	case "FATAL":
		// FatalLevel logs a message, then calls os.Exit(1).
		loglevel.SetLevel(zapcore.FatalLevel)
	default:
		// default loglevel is INFO
		loglevel.SetLevel(zapcore.InfoLevel)
	}
}

func PanicHandler(stderr string) {
	zap.S().Sync()
	loggerName := zap.L().Name()
	panicLog := PanicLog{
		RegDate:     time.Now().Format(time.RFC3339),
		ServiceName: loggerName,
		LogLevel:    "PANIC"}

	if _, ok := zapCoreMap["logClient"]; ok {
		panicLogMessage := strings.Split(stderr, "\n\n")
		if len(panicLogMessage) == 2 {
			panicLog.Message = panicLogMessage[0]
			panicLog.StackTrace = panicLogMessage[1]
		} else {
			panicLog.Message = stderr
		}
		jsonStr, err := json.Marshal(panicLog)
		if err == nil {
			lc.(LogClientSink).SendPanicLog(string(jsonStr))
		}
	}

	lumberjackCore, ok := zapCoreMap["lumberjack"]
	if ok {
		zap.L().Name()
		logger := zap.New(lumberjackCore, zap.AddCaller())
		logger = logger.Named(loggerName)
		logger.DPanic(stderr)
		logger.Sync()
	}
}
