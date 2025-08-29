package cron

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

// DefaultLogger 如果未指定，则由Cron使用。
var DefaultLogger Logger = PrintfLogger(log.New(os.Stdout, "cron: ", log.LstdFlags))

// DiscardLogger 可以被调用者用来丢弃所有日志消息。
var DiscardLogger Logger = PrintfLogger(log.New(ioutil.Discard, "", 0))

// Logger 是此包中用于日志记录的接口，因此可以插入任何后端。
// 它是github.com/go-logr/logr接口的子集。
type Logger interface {
	// Info 记录关于cron操作的常规消息。
	Info(msg string, keysAndValues ...interface{})
	// Error 记录错误条件。
	Error(err error, msg string, keysAndValues ...interface{})
}

// PrintfLogger 将基于Printf的记录器（如标准库"log"）
// 包装成仅记录错误的Logger接口实现。
func PrintfLogger(l interface{ Printf(string, ...interface{}) }) Logger {
	return printfLogger{l, false}
}

// VerbosePrintfLogger 将基于Printf的记录器（如标准库"log"）
// 包装成记录所有内容的Logger接口实现。
func VerbosePrintfLogger(l interface{ Printf(string, ...interface{}) }) Logger {
	return printfLogger{l, true}
}

type printfLogger struct {
	logger  interface{ Printf(string, ...interface{}) }
	logInfo bool
}

func (pl printfLogger) Info(msg string, keysAndValues ...interface{}) {
	if pl.logInfo {
		keysAndValues = formatTimes(keysAndValues)
		pl.logger.Printf(
			formatString(len(keysAndValues)),
			append([]interface{}{msg}, keysAndValues...)...)
	}
}

func (pl printfLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	keysAndValues = formatTimes(keysAndValues)
	pl.logger.Printf(
		formatString(len(keysAndValues)+2),
		append([]interface{}{msg, "error", err}, keysAndValues...)...)
}

// formatString 为键/值的数量返回类似logfmt的格式字符串。
func formatString(numKeysAndValues int) string {
	var sb strings.Builder
	sb.WriteString("%s")
	if numKeysAndValues > 0 {
		sb.WriteString(", ")
	}
	for i := 0; i < numKeysAndValues/2; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("%v=%v")
	}
	return sb.String()
}

// formatTimes 将任何time.Time值格式化为RFC3339。
func formatTimes(keysAndValues []interface{}) []interface{} {
	var formattedArgs []interface{}
	for _, arg := range keysAndValues {
		if t, ok := arg.(time.Time); ok {
			arg = t.Format(time.RFC3339)
		}
		formattedArgs = append(formattedArgs, arg)
	}
	return formattedArgs
}
