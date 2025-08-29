package cron

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/go-utils2/time2"
)

// Cron 跟踪任意数量的条目，按照计划调用相关的函数。
// 它可以被启动、停止，并且可以在运行时检查条目。
type Cron struct {
	entries   []*Entry
	chain     Chain
	stop      chan struct{}
	add       chan *Entry
	remove    chan EntryID
	snapshot  chan chan []Entry
	running   bool
	logger    Logger
	runningMu sync.Mutex
	location  *time.Location
	parser    ScheduleParser
	nextID    EntryID
	jobWaiter sync.WaitGroup
}

// ScheduleParser 是用于解析计划规范并返回 Schedule 的接口
type ScheduleParser interface {
	Parse(spec string) (Schedule, error)
}

// Job 是提交的 cron 作业的接口。
type Job interface {
	Run()
}

// Schedule 描述作业的执行周期。
type Schedule interface {
	// Next 返回下一个激活时间，晚于给定时间。
	// Next 最初被调用，然后在每次作业运行时被调用。
	Next(time.Time) time.Time
}

// EntryID 标识 Cron 实例中的条目
type EntryID int

// Entry 由计划和在该计划上执行的函数组成。
type Entry struct {
	// ID 是此条目的 cron 分配的 ID，可用于查找快照或删除它。
	ID EntryID

	// Schedule 是此作业应该运行的计划。
	Schedule Schedule

	// Next 是作业将运行的下一次时间，如果 Cron 尚未启动或此条目的计划不可满足，则为零时间
	Next time.Time

	// Prev 是此作业上次运行的时间，如果从未运行则为零时间。
	Prev time.Time

	// WrappedJob 是当 Schedule 被激活时要运行的东西。
	WrappedJob Job

	// Job 是提交给 cron 的东西。
	// 保留它是为了让需要稍后获取作业的用户代码（例如通过 Entries()）可以这样做。
	Job Job
}

// Valid 如果这不是零条目则返回 true。
func (e Entry) Valid() bool { return e.ID != 0 }

// byTime 是用于按时间排序条目数组的包装器
// （零时间在末尾）。
type byTime []*Entry

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	// 两个零时间应该返回 false。
	// 否则，零时间比任何其他时间都"大"。
	//（将其排序到列表的末尾。）
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}

// New 返回一个新的 Cron 作业运行器，由给定的选项修改。
//
// 可用设置
//
//	时区
//	  描述: 解释计划的时区
//	  默认值:     time.Local
//
//	解析器
//	  描述: 解析器将 cron 规范字符串转换为 cron.Schedules。
//	  默认值:     接受此规范: https://en.wikipedia.org/wiki/Cron
//
//	链
//	  描述: 包装提交的作业以自定义行为。
//	  默认值:     一个恢复恐慌并将其记录到 stderr 的链。
//
// 请参阅 "cron.With*" 来修改默认行为。
func New(opts ...Option) *Cron {
	c := &Cron{
		entries:   nil,
		chain:     NewChain(),
		add:       make(chan *Entry),
		stop:      make(chan struct{}),
		snapshot:  make(chan chan []Entry),
		remove:    make(chan EntryID),
		running:   false,
		runningMu: sync.Mutex{},
		logger:    DefaultLogger,
		location:  time.Local,
		parser:    standardParser,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FuncJob 是将 func() 转换为 cron.Job 的包装器
type FuncJob func()

func (f FuncJob) Run() { f() }

// AddFunc 向 Cron 添加一个函数，以在给定的计划上运行。
// 使用此 Cron 实例的时区作为默认值来解析规范。
// 返回一个不透明的 ID，可用于稍后删除它。
func (c *Cron) AddFunc(spec string, cmd func()) (EntryID, error) {
	return c.AddJob(spec, FuncJob(cmd))
}

// AddJob 向 Cron 添加一个 Job，以在给定的计划上运行。
// 使用此 Cron 实例的时区作为默认值来解析规范。
// 返回一个不透明的 ID，可用于稍后删除它。
func (c *Cron) AddJob(spec string, cmd Job) (EntryID, error) {
	schedule, err := c.parser.Parse(spec)
	if err != nil {
		return 0, err
	}
	return c.Schedule(schedule, cmd), nil
}

// Schedule 向 Cron 添加一个 Job，以在给定的计划上运行。
// 作业使用配置的链进行包装。
func (c *Cron) Schedule(schedule Schedule, cmd Job) EntryID {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	c.nextID++
	entry := &Entry{
		ID:         c.nextID,
		Schedule:   schedule,
		WrappedJob: c.chain.Then(cmd),
		Job:        cmd,
	}
	if !c.running {
		c.entries = append(c.entries, entry)
	} else {
		c.add <- entry
	}
	return entry.ID
}

// Entries 返回 cron 条目的快照。
func (c *Cron) Entries() []Entry {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		replyChan := make(chan []Entry, 1)
		c.snapshot <- replyChan
		return <-replyChan
	}
	return c.entrySnapshot()
}

// Location 获取时区位置
func (c *Cron) Location() *time.Location {
	return c.location
}

// Entry 返回给定条目的快照，如果找不到则返回 nil。
func (c *Cron) Entry(id EntryID) Entry {
	for _, entry := range c.Entries() {
		if id == entry.ID {
			return entry
		}
	}
	return Entry{}
}

// Remove 从将来运行中删除一个条目。
func (c *Cron) Remove(id EntryID) {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.remove <- id
	} else {
		c.removeEntry(id)
	}
}

// Start 在自己的 goroutine 中启动 cron 调度器，如果已经启动则为无操作。
func (c *Cron) Start() {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

// Run 运行 cron 调度器，如果已经运行则为无操作。
func (c *Cron) Run() {
	c.runningMu.Lock()
	if c.running {
		c.runningMu.Unlock()
		return
	}
	c.running = true
	c.runningMu.Unlock()
	c.run()
}

// run 运行调度器.. 这是私有的，只是因为需要同步
// 对 'running' 状态变量的访问。
func (c *Cron) run() {
	c.logger.Info("start")

	// 计算每个条目的下一次激活时间。
	now := c.now()
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
		c.logger.Info("schedule", "now", now, "entry", entry.ID, "next", entry.Next)
	}

	for {
		// 确定要运行的下一个条目。
		sort.Sort(byTime(c.entries))

		var timer *time.Timer
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// 如果还没有条目，就睡眠 - 它仍然处理新条目
			// 和停止请求。
			timer = time.NewTimer(100000 * time.Hour)
		} else {
			timer = time.NewTimer(c.entries[0].Next.Sub(now))
		}

		for {
			select {
			case now = <-timer.C:
				now = now.In(c.location)
				c.logger.Info("wake", "now", now)

				// 运行下一次时间小于现在的每个条目
				for _, e := range c.entries {
					if e.Next.After(now) || e.Next.IsZero() {
						break
					}
					c.startJob(e.WrappedJob)
					e.Prev = e.Next
					e.Next = e.Schedule.Next(now)
					c.logger.Info("run", "now", now, "entry", e.ID, "next", e.Next)
				}

			case newEntry := <-c.add:
				timer.Stop()
				now = c.now()
				newEntry.Next = newEntry.Schedule.Next(now)
				c.entries = append(c.entries, newEntry)
				c.logger.Info("added", "now", now, "entry", newEntry.ID, "next", newEntry.Next)

			case replyChan := <-c.snapshot:
				replyChan <- c.entrySnapshot()
				continue

			case <-c.stop:
				timer.Stop()
				c.logger.Info("stop")
				return

			case id := <-c.remove:
				timer.Stop()
				now = c.now()
				c.removeEntry(id)
				c.logger.Info("removed", "entry", id)
			}

			break
		}
	}
}

// startJob 在新的 goroutine 中运行给定的作业。
func (c *Cron) startJob(j Job) {
	c.jobWaiter.Add(1)
	go func() {
		defer c.jobWaiter.Done()
		j.Run()
	}()
}

// now 返回 c 位置的当前时间
func (c *Cron) now() time.Time {
	return time2.Now().In(c.location)
}

// Stop 如果 cron 调度器正在运行则停止它；否则什么也不做。
// 返回一个上下文，以便调用者可以等待正在运行的作业完成。
func (c *Cron) Stop() context.Context {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.stop <- struct{}{}
		c.running = false
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.jobWaiter.Wait()
		cancel()
	}()
	return ctx
}

// entrySnapshot 返回当前 cron 条目列表的副本。
func (c *Cron) entrySnapshot() []Entry {
	var entries = make([]Entry, len(c.entries))
	for i, e := range c.entries {
		entries[i] = *e
	}
	return entries
}

func (c *Cron) removeEntry(id EntryID) {
	var entries []*Entry
	for _, e := range c.entries {
		if e.ID != id {
			entries = append(entries, e)
		}
	}
	c.entries = entries
}
