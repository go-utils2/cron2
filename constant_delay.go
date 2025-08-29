package cron

import "time"

// ConstantDelaySchedule 表示简单的重复工作周期，例如"每5分钟"。
// 它不支持频率超过每秒一次的作业。
type ConstantDelaySchedule struct {
	Delay time.Duration
}

// Every 返回一个每隔duration激活一次的crontab调度。
// 不支持小于一秒的延迟（将向上舍入到1秒）。
// 任何小于秒的字段都会被截断。
func Every(duration time.Duration) ConstantDelaySchedule {
	if duration < time.Second {
		duration = time.Second
	}
	return ConstantDelaySchedule{
		Delay: duration - time.Duration(duration.Nanoseconds())%time.Second,
	}
}

// Next 返回下次应该运行的时间。
// 这会进行舍入，使下次激活时间在秒上。
func (schedule ConstantDelaySchedule) Next(t time.Time) time.Time {
	return t.Add(schedule.Delay - time.Duration(t.Nanosecond())*time.Nanosecond)
}
