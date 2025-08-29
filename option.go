package cron

import (
	"time"
)

// Option 表示对Cron默认行为的修改。
type Option func(*Cron)

// WithLocation 覆盖cron实例的时区。
func WithLocation(loc *time.Location) Option {
	return func(c *Cron) {
		c.location = loc
	}
}

// WithSeconds 覆盖用于解释作业调度的解析器，
// 将秒字段作为第一个字段包含在内。
func WithSeconds() Option {
	return WithParser(NewParser(
		Second | Minute | Hour | Dom | Month | Dow | Descriptor,
	))
}

// WithParser 覆盖用于解释作业调度的解析器。
func WithParser(p ScheduleParser) Option {
	return func(c *Cron) {
		c.parser = p
	}
}

// WithChain 指定要应用于添加到此cron的所有作业的作业包装器。
// 请参考此包中的Chain*函数以获取提供的包装器。
func WithChain(wrappers ...JobWrapper) Option {
	return func(c *Cron) {
		c.chain = NewChain(wrappers...)
	}
}

// WithLogger 使用提供的记录器。
func WithLogger(logger Logger) Option {
	return func(c *Cron) {
		c.logger = logger
	}
}
