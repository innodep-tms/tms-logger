package tmslogger

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type LogClient struct {
	mutex           *sync.Mutex
	ServiceId       string
	ServiceVersion  string
	client          *resty.Client
	host            *url.URL
	logChan         chan string
	chanCapacity    int
	maxChanSize     int
	sendLogBuffer   []string
	sendLogBufSize  int
	maxLogBufSize   int
	chanOverflowCnt int64
	sendFailCnt     int64
}

type LogClientSink struct {
	*LogClient
}

// hostString
//	- ntms-log-service host string
// serviceVersion
// 	- service version
// chanSize
// 	- channel buffer size. default 10MB
// sendBufSize
// 	- send buffer size default 100KB
func NewLogClient(hostString string, serviceVersion string, chanSize int, sendBufSize int) (zap.Sink, error) {
	var defaultChanSize = 1024 * 1024 * 10
	var defaultSendBufSize = 1024 * 100

	if chanSize != 0 {
		defaultChanSize = chanSize
	}
	if sendBufSize != 0 {
		defaultSendBufSize = sendBufSize
	}
	if defaultSendBufSize > defaultChanSize {
		defaultSendBufSize = defaultChanSize
	}

	host, err := url.Parse(hostString)
	if err != nil {
		return nil, err
	}

	sink := LogClientSink{
		LogClient: &LogClient{
			mutex:           &sync.Mutex{},
			ServiceId:       uuid.New().String(),
			ServiceVersion:  serviceVersion,
			client:          resty.New(),
			host:            host,
			logChan:         make(chan string),
			maxChanSize:     defaultChanSize,    // 채널버퍼 최대 용량(한계치 이상은 누적 불가)
			chanCapacity:    0,                  // 채널버퍼 현재 용량
			sendLogBuffer:   make([]string, 0),  // 전송할 로그를 묶어서 전송하기 위한 버퍼
			sendLogBufSize:  0,                  // 전송할 로그 버퍼에 쌓여 있는 로그 용량
			maxLogBufSize:   defaultSendBufSize, // 전송할 로그 버퍼에 최대로 쌓을 수 있는 로그 용량
			chanOverflowCnt: 0,                  // 채널버퍼 한계치 오버로 버려진 로그 누적 수량
			sendFailCnt:     0,                  // log service 에 api 호출 실패 수량
		},
	}

	go sink.emit()

	return sink, nil
}

func (lc *LogClient) Write(p []byte) (n int, err error) {
	if len(p)+lc.chanCapacity < lc.maxChanSize {
		lc.chanCapacity += len(p)
		lc.logChan <- string(p)
	} else {
		lc.chanOverflowCnt++
	}
	return len(p), nil
}

func (lc *LogClient) Sync() error {
	lc.sendLog(0)
	return nil
}

func (lc *LogClient) Close() error {
	close(lc.logChan)
	return nil
}

func (lc *LogClient) emit() {
	for {
		select {
		case rawLogData, more := <-lc.logChan:
			if more {
				lc.appendLogBuffer(rawLogData)
				lc.sendLog(lc.maxLogBufSize)
			} else {
				// 채널이 닫혔으므로 종료
				lc.sendLog(0)
				break
			}
		case <-time.After(10 * time.Millisecond):
			// 샌드버퍼에 남아 있는 내용을 전송
			lc.sendLog(0)
		}
	}
}

func (lc *LogClient) appendLogBuffer(logmessage string) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	lc.sendLogBuffer = append(lc.sendLogBuffer, logmessage)
	lc.sendLogBufSize += len(logmessage)
	lc.chanCapacity -= len(logmessage)
}

func (lc *LogClient) sendLog(limitSize int) {
	tmpSendBuffer := ""
	tmpSendBufSize := 0
	sendCnt := 0
	lc.mutex.Lock()
	if lc.sendLogBufSize > limitSize {
		sendCnt = len(lc.sendLogBuffer)
		tmpSendBuffer = strings.Join(lc.sendLogBuffer, "")
		tmpSendBufSize = lc.sendLogBufSize
		lc.sendLogBufSize = 0
		lc.sendLogBuffer = make([]string, 0)
	}
	lc.mutex.Unlock()

	if tmpSendBufSize > limitSize {
		res, err := lc.client.SetTimeout(time.Second).R().
			SetHeaders(map[string]string{
				"Content-Type":      "application/x-ndjson",
				"X-Service-Id":      lc.ServiceId,
				"X-Service-Version": lc.ServiceVersion,
			}).
			SetBody(tmpSendBuffer).
			Post(lc.host.String())
		if res.StatusCode() != 200 || err != nil {
			// 전송실패
			lc.sendFailCnt += int64(sendCnt)
		}
	}
}

func (lc *LogClient) SendPanicLog(msg string) {
	res, err := lc.client.SetTimeout(time.Second).R().
		SetHeaders(map[string]string{
			"Content-Type":      "application/x-ndjson",
			"X-Service-Id":      lc.ServiceId,
			"X-Service-Version": lc.ServiceVersion,
		}).
		SetBody(msg).
		Post(lc.host.String())
	if res.StatusCode() != 200 || err != nil {
		// 전송실패
		lc.sendFailCnt += 1
	}
}

func (lc *LogClient) GetStatic() (int64, int64) {
	return lc.chanOverflowCnt, lc.sendFailCnt
}
