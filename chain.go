package cron

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/go-utils2/time2"
)

// JobWrapper 用某些行为装饰给定的Job。
type JobWrapper func(Job) Job

// Chain 是JobWrapper的序列，用横切行为（如日志记录或同步）
// 装饰提交的作业。
type Chain struct {
	wrappers []JobWrapper
}

// NewChain 返回由给定JobWrapper组成的Chain。
func NewChain(c ...JobWrapper) Chain {
	return Chain{c}
}

// Then 用链中的所有JobWrapper装饰给定的作业。
//
// 这样：
//
//	NewChain(m1, m2, m3).Then(job)
//
// 等价于：
//
//	m1(m2(m3(job)))
func (c Chain) Then(j Job) Job {
	for i := range c.wrappers {
		j = c.wrappers[len(c.wrappers)-i-1](j)
	}
	return j
}

// Recover 恢复包装作业中的panic并使用提供的记录器记录它们。
func Recover(logger Logger) JobWrapper {
	return func(j Job) Job {
		return FuncJob(func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					logger.Error(err, "panic", "stack", "...\n"+string(buf))
				}
			}()
			j.Run()
		})
	}
}

// DelayIfStillRunning 序列化作业，延迟后续运行直到前一个完成。
// 延迟超过一分钟后运行的作业会在Info级别记录延迟。
func DelayIfStillRunning(logger Logger) JobWrapper {
	return func(j Job) Job {
		var mu sync.Mutex
		return FuncJob(func() {
			start := time2.Now()
			mu.Lock()
			defer mu.Unlock()
			if dur := time.Since(start); dur > time.Minute {
				logger.Info("delay", "duration", dur)
			}
			j.Run()
		})
	}
}

// SkipIfStillRunning 如果前一个调用仍在运行，则跳过Job的调用。
// 它在Info级别向给定记录器记录跳过。
func SkipIfStillRunning(logger Logger) JobWrapper {
	return func(j Job) Job {
		var ch = make(chan struct{}, 1)
		ch <- struct{}{}
		return FuncJob(func() {
			select {
			case v := <-ch:
				defer func() { ch <- v }()
				j.Run()
			default:
				logger.Info("skip")
			}
		})
	}
}
