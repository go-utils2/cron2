package cron

import "time"

// SpecSchedule 指定基于传统crontab规范的工作周期（精确到秒）。
// 它最初被计算并存储为位集。
type SpecSchedule struct {
	Second, Minute, Hour, Dom, Month, Dow uint64

	// 覆盖此调度的位置。
	Location *time.Location
}

// bounds 提供可接受值的范围（加上名称到值的映射）。
type bounds struct {
	min, max uint
	names    map[string]uint
}

// 每个字段的边界。
var (
	seconds = bounds{0, 59, nil}
	minutes = bounds{0, 59, nil}
	hours   = bounds{0, 23, nil}
	dom     = bounds{1, 31, nil}
	months  = bounds{1, 12, map[string]uint{
		"jan": 1,
		"feb": 2,
		"mar": 3,
		"apr": 4,
		"may": 5,
		"jun": 6,
		"jul": 7,
		"aug": 8,
		"sep": 9,
		"oct": 10,
		"nov": 11,
		"dec": 12,
	}}
	dow = bounds{0, 6, map[string]uint{
		"sun": 0,
		"mon": 1,
		"tue": 2,
		"wed": 3,
		"thu": 4,
		"fri": 5,
		"sat": 6,
	}}
)

const (
	// 如果表达式中包含星号，则设置最高位。
	starBit = 1 << 63
)

// Next 返回此调度激活的下一个时间，大于给定时间。
// 如果找不到满足调度的时间，则返回零时间。
func (s *SpecSchedule) Next(t time.Time) time.Time {
	// 一般方法
	//
	// 对于月、日、时、分、秒：
	// 检查时间值是否匹配。如果是，继续到下一个字段。
	// 如果字段不匹配调度，则递增字段直到匹配。
	// 在递增字段时，环绕会将其带回字段列表的开头
	//（因为需要重新验证先前的字段值）

	// 如果指定了时区，将给定时间转换为调度的时区。
	// 保存原始时区，以便在找到时间后可以转换回来。
	// 请注意，未指定时区的调度（time.Local）被视为
	// 提供时间的本地时区。
	origLocation := t.Location()
	loc := s.Location
	if loc == time.Local {
		loc = t.Location()
	}
	if s.Location != time.Local {
		t = t.In(s.Location)
	}

	// 从最早可能的时间开始（即将到来的秒）。
	t = t.Add(1*time.Second - time.Duration(t.Nanosecond())*time.Nanosecond)

	// 此标志指示字段是否已递增。
	added := false

	// 如果在五年内找不到时间，则返回零。
	yearLimit := t.Year() + 5

WRAP:
	if t.Year() > yearLimit {
		return time.Time{}
	}

	// 找到第一个适用的月份。
	// 如果是这个月，则什么都不做。
	for 1<<uint(t.Month())&s.Month == 0 {
		// 如果我们必须添加一个月，将其他部分重置为0。
		if !added {
			added = true
			// 否则，将日期设置在开头（因为当前时间无关紧要）。
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		}
		t = t.AddDate(0, 1, 0)

		// 环绕。
		if t.Month() == time.January {
			goto WRAP
		}
	}

	// 现在获取该月的一天。
	//
	// 注意：这会对夏令时制度造成问题，其中午夜不存在。
	// 例如：圣保罗的夏令时将11/3的午夜转换为凌晨1点。
	// 通过注意小时何时不等于0来处理这种情况。
	for !dayMatches(s, t) {
		if !added {
			added = true
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
		t = t.AddDate(0, 0, 1)
		// 注意由于夏令时，小时是否不再是午夜。
		// 如果是23，则加一小时；如果是1，则减一小时。
		if t.Hour() != 0 {
			if t.Hour() > 12 {
				t = t.Add(time.Duration(24-t.Hour()) * time.Hour)
			} else {
				t = t.Add(time.Duration(-t.Hour()) * time.Hour)
			}
		}

		if t.Day() == 1 {
			goto WRAP
		}
	}

	for 1<<uint(t.Hour())&s.Hour == 0 {
		if !added {
			added = true
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)
		}
		t = t.Add(1 * time.Hour)

		if t.Hour() == 0 {
			goto WRAP
		}
	}

	for 1<<uint(t.Minute())&s.Minute == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Minute)
		}
		t = t.Add(1 * time.Minute)

		if t.Minute() == 0 {
			goto WRAP
		}
	}

	for 1<<uint(t.Second())&s.Second == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Second)
		}
		t = t.Add(1 * time.Second)

		if t.Second() == 0 {
			goto WRAP
		}
	}

	return t.In(origLocation)
}

// dayMatches 如果给定时间满足调度的星期几和月中的日
// 限制，则返回true。
func dayMatches(s *SpecSchedule, t time.Time) bool {
	var (
		domMatch bool = 1<<uint(t.Day())&s.Dom > 0
		dowMatch bool = 1<<uint(t.Weekday())&s.Dow > 0
	)
	if s.Dom&starBit > 0 || s.Dow&starBit > 0 {
		return domMatch && dowMatch
	}
	return domMatch || dowMatch
}
