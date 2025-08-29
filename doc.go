/*
Package cron 实现了一个 cron 规范解析器和作业运行器。

# 安装

要下载特定的标记版本，请运行：

	go get github.com/go-utils2/cron2@latest

在您的程序中导入：

	import "github.com/go-utils2/cron2"

由于使用了 Go Modules，它需要 Go 1.24.5 或更高版本。

# 使用

调用者可以注册函数以在给定的计划上调用。Cron 将在它们自己的 goroutine 中运行它们。

	c := cron.New()
	c.AddFunc("30 * * * *", func() { fmt.Println("每小时的半点") })
	c.AddFunc("30 3-6,20-23 * * *", func() { fmt.Println(".. 在凌晨3-6点，晚上8-11点的范围内") })
	c.AddFunc("CRON_TZ=Asia/Tokyo 30 04 * * *", func() { fmt.Println("每天东京时间04:30运行") })
	c.AddFunc("@hourly",      func() { fmt.Println("每小时，从现在开始一小时后") })
	c.AddFunc("@every 1h30m", func() { fmt.Println("每一小时三十分钟，从现在开始一小时三十分钟后") })
	c.Start()
	..
	// 函数在它们自己的 goroutine 中异步调用。
	...
	// 函数也可以添加到正在运行的 Cron 中
	c.AddFunc("@daily", func() { fmt.Println("每天") })
	..
	// 检查 cron 作业条目的下次和上次运行时间。
	inspect(c.Entries())
	..
	c.Stop()  // 停止调度器（不会停止任何已经运行的作业）。

# CRON 表达式格式

cron 表达式表示一组时间，使用 5 个空格分隔的字段。

	字段名称     | 必需？     | 允许的值        | 允许的特殊字符
	----------   | ---------- | --------------  | --------------------------
	分钟         | 是         | 0-59            | * / , -
	小时         | 是         | 0-23            | * / , -
	月中的日     | 是         | 1-31            | * / , - ?
	月份         | 是         | 1-12 或 JAN-DEC | * / , -
	星期几       | 是         | 0-6 或 SUN-SAT  | * / , - ?

月份和星期几字段值不区分大小写。"SUN"、"Sun" 和 "sun" 都同样被接受。

格式的具体解释基于 Cron 维基百科页面：
https://en.wikipedia.org/wiki/Cron

# 替代格式

替代的 Cron 表达式格式支持其他字段，如秒。您可以通过创建自定义解析器来实现，如下所示。

	cron.New(
		cron.WithParser(
			cron.NewParser(
				cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)))

由于添加秒是对标准 cron 规范最常见的修改，cron 提供了一个内置函数来实现这一点，
它等同于您之前看到的自定义解析器，除了它的秒字段是必需的：

	cron.New(cron.WithSeconds())

这模拟了 Quartz，最流行的替代 Cron 调度格式：
http://www.quartz-scheduler.org/documentation/quartz-2.x/tutorials/crontrigger.html

# 特殊字符

星号 ( * )

星号表示 cron 表达式将匹配字段的所有值；例如，在第5个字段（月份）中使用星号
将表示每个月。

斜杠 ( / )

斜杠用于描述范围的增量。例如，第1个字段（分钟）中的 3-59/15 将表示
小时的第3分钟以及此后每15分钟。形式 "*\/..." 等同于形式 "first-last/...",
即在字段的最大可能范围上的增量。形式 "N/..." 被接受为意思是 "N-MAX/...",
即从 N 开始，使用增量直到该特定范围的结束。它不会环绕。

逗号 ( , )

逗号用于分隔列表中的项目。例如，在第5个字段（星期几）中使用 "MON,WED,FRI"
将意味着星期一、星期三和星期五。

连字符 ( - )

连字符用于定义范围。例如，9-17 将表示上午9点到下午5点之间的每个小时（包括两端）。

问号 ( ? )

问号可以用来代替 '*' 来留空月中的日或星期几。

# 预定义计划

您可以使用几个预定义的计划之一来代替 cron 表达式。

	条目                   | 描述                                       | 等同于
	-----                  | -----------                                | -------------
	@yearly (或 @annually) | 每年运行一次，午夜，1月1日                 | 0 0 1 1 *
	@monthly               | 每月运行一次，午夜，月初                   | 0 0 1 * *
	@weekly                | 每周运行一次，周六/周日之间的午夜           | 0 0 * * 0
	@daily (或 @midnight)  | 每天运行一次，午夜                         | 0 0 * * *
	@hourly                | 每小时运行一次，小时开始时                 | 0 * * * *

# 间隔

您还可以安排作业以固定间隔执行，从添加时间或运行 cron 时开始。
这通过如下格式化 cron 规范来支持：

	@every <duration>

其中 "duration" 是 time.ParseDuration 接受的字符串
(http://golang.org/pkg/time/#ParseDuration)。

例如，"@every 1h30m10s" 将表示在1小时30分钟10秒后激活的计划，
然后在此后的每个间隔。

注意：间隔不考虑作业运行时间。例如，如果作业需要3分钟运行，
并且计划每5分钟运行一次，它在每次运行之间只有2分钟的空闲时间。

# 时区

默认情况下，所有解释和调度都在机器的本地时区（time.Local）中完成。您可以在构造时指定不同的时区：

	cron.New(
	    cron.WithLocation(time.UTC))

单个 cron 计划也可以通过在 cron 规范的开头提供额外的空格分隔字段来覆盖它们要解释的时区，
格式为 "CRON_TZ=Asia/Tokyo"。

例如：

	# 在 time.Local 的上午6点运行
	cron.New().AddFunc("0 6 * * ?", ...)

	# 在 America/New_York 的上午6点运行
	nyc, _ := time.LoadLocation("America/New_York")
	c := cron.New(cron.WithLocation(nyc))
	c.AddFunc("0 6 * * ?", ...)

	# 在 Asia/Tokyo 的上午6点运行
	cron.New().AddFunc("CRON_TZ=Asia/Tokyo 0 6 * * ?", ...)

	# 在 Asia/Tokyo 的上午6点运行
	c := cron.New(cron.WithLocation(nyc))
	c.SetLocation("America/New_York")
	c.AddFunc("CRON_TZ=Asia/Tokyo 0 6 * * ?", ...)

前缀 "TZ=(TIME ZONE)" 也支持用于传统兼容性。

请注意，在夏令时跳跃转换期间安排的作业将不会运行！

# 作业包装器

Cron 运行器可以配置一系列作业包装器，为所有提交的作业添加横切功能。
例如，它们可以用于实现以下效果：

  - 从作业中恢复任何恐慌（默认激活）
  - 如果前一次运行尚未完成，则延迟作业的执行
  - 如果前一次运行尚未完成，则跳过作业的执行
  - 记录每个作业的调用
  - 作业完成时的通知

使用 `cron.WithChain` 选项为添加到 cron 的所有作业安装包装器：

	c := cron.New(cron.WithChain(
		cron.DelayIfStillRunning(cron.DefaultLogger),
		cron.Recover(cron.DefaultLogger),
	))

仅为某些作业使用 `Chain` 方法安装包装器：

	c := cron.New()
	c.AddJob("@every 30s", cron.NewChain(
		cron.SkipIfStillRunning(cron.DefaultLogger),
	).Then(job))

作业包装器按照它们定义的顺序调用，因此 `Recover` 包装器通常应该最后出现
（或者如果您希望恐慌对后续包装器可见，则在链的早期）。

# 线程安全

由于 Cron 服务与调用代码并发运行，必须采取一定的注意措施来确保正确的同步。

所有 cron 方法都设计为正确同步，只要调用者确保调用之间有明确的先行发生排序。

# 日志记录

Cron 定义了一个 Logger 接口，它是 github.com/go-logr/logr 中定义的接口的子集。
它有两个日志级别（Info 和 Error），参数是键/值对。这使得 cron 日志记录可以插入
结构化日志系统。提供了一个适配器 [Verbose]PrintfLogger 来包装标准库 *log.Logger。

为了更深入地了解 Cron 操作，可以激活详细日志记录，它将记录作业运行、调度决策
以及添加或删除的作业。使用一次性记录器激活它，如下所示：

	cron.New(
		cron.WithLogger(
			cron.VerbosePrintfLogger(log.New(os.Stdout, "cron: ", log.LstdFlags))))

Cron 默认不需要或提供任何日志记录。如上所述启用它或根据您的喜好提供不同的记录器。

# 实现

Cron 条目存储在一个数组中，按其下次激活时间排序。Cron 睡眠直到下一个作业应该运行。

唤醒时：
  - 它运行在该秒钟活跃的每个条目
  - 它计算已运行作业的下次运行时间
  - 它按下次激活时间重新排序条目数组
  - 它睡眠直到最早的作业
*/
package cron
