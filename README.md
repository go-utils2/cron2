[![GoDoc](http://godoc.org/github.com/robfig/cron?status.png)](http://godoc.org/github.com/robfig/cron)
[![Build Status](https://travis-ci.org/robfig/cron.svg?branch=master)](https://travis-ci.org/robfig/cron)

# cron

Cron V3 已经发布！

要下载特定的标记版本，请运行：
```bash
go get github.com/go-utils2/cron2@latest
```
在您的程序中导入：
```go
import "github.com/go-utils2/cron2"
```
由于使用了 Go Modules，它需要 Go 1.24.5 或更高版本。

请参考这里的文档：
http://godoc.org/github.com/go-utils2/cron2

本文档的其余部分描述了 v3 的改进以及希望从早期版本升级的用户的重大变更列表。

## 升级到 v3 (2019年6月)

cron v3 是对库的重大升级，解决了所有未解决的错误、功能请求和粗糙边缘。它基于主分支的合并，
主分支包含多年来发现的各种问题的修复，以及 v2 分支，该分支包含一些向后不兼容的功能，
如删除 cron 作业的能力。此外，v3 增加了对 Go Modules 的支持，清理了时区支持等粗糙边缘，
并修复了许多错误。

新功能：

- 支持 Go modules。调用者现在必须将此库导入为
  `github.com/go-utils2/cron2`，而不是 `gopkg.in/...`

- 修复的错误：
  - 0f01e6b parser: 修复 Dow 和 Dom 的组合 (#70)
  - dbf3220 在向前滚动时钟时调整时间以处理不存在的午夜 (#157)
  - eeecf15 spec_test.go: 确保在 0 增量时返回错误 (#144)
  - 70971dc cron.Entries(): 更新快照请求以包含回复通道 (#97)
  - 1cba5e6 cron: 修复：删除作业导致下一个计划作业运行过晚 (#206)

- 默认使用标准 cron 规范解析（第一个字段是"分钟"），并提供简单的方式
  选择秒字段（与 quartz 兼容）。但是，请注意不支持年字段（在 Quartz 中是可选的）。

- 通过符合 https://github.com/go-logr/logr 项目的接口进行可扩展的键/值日志记录。

- 新的 Chain 和 JobWrapper 类型允许您安装"拦截器"来添加
  以下横切行为：
  - 从作业中恢复任何 panic
  - 如果前一次运行尚未完成，则延迟作业的执行
  - 如果前一次运行尚未完成，则跳过作业的执行
  - 记录每个作业的调用
  - 作业完成时的通知

它与 v1 和 v2 都不向后兼容。需要进行以下更新：

- v1 分支在 cron 规范的开头接受可选的秒字段。这是非标准的，导致了很多混乱。
  新的默认解析器符合 [Cron 维基百科页面] 描述的标准。

  更新：要保留旧行为，请使用自定义解析器构造您的 Cron：
```go
// 秒字段，必需
cron.New(cron.WithSeconds())

// 秒字段，可选
cron.New(cron.WithParser(cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)))
```
- Cron 类型现在在构造时接受函数选项，而不是以前的临时行为修改机制
  （设置字段、调用设置器）。

  更新：设置 Cron.ErrorLogger 或调用 Cron.SetLocation 的代码必须
  更新为在构造时提供这些值。

- CRON_TZ 现在是指定单个计划时区的推荐方式，这得到了规范的认可。
  传统的 "TZ=" 前缀将继续得到支持，因为它是明确的且易于实现。

  更新：无需更新。

- 默认情况下，cron 将不再恢复它运行的作业中的 panic。
  恢复可能令人惊讶（参见问题 #192），似乎与库的典型行为不符。
  相关地，`cron.WithPanicLogger` 选项已被删除，以适应更通用的 JobWrapper 类型。

  更新：要选择 panic 恢复并配置 panic 记录器：
```go
cron.New(cron.WithChain(
  cron.Recover(logger),  // 或使用 cron.DefaultLogger
))
```
- 在添加对 https://github.com/go-logr/logr 的支持时，`cron.WithVerboseLogger` 被
  删除，因为它与分级日志记录重复。

  更新：调用者应使用 `WithLogger` 并指定不丢弃 `Info` 日志的记录器。
  为方便起见，提供了一个包装 `*log.Logger` 的记录器：
```go
cron.New(
  cron.WithLogger(cron.VerbosePrintfLogger(logger)))
```

### 背景 - Cron 规范格式

常用的 cron 规范格式有两种：

- "标准" cron 格式，在 [Cron 维基百科页面] 上描述，由 cron Linux 系统实用程序使用。

- [Quartz 调度器] 使用的 cron 格式，通常用于 Java 软件中的计划作业

[the Cron wikipedia page]: https://en.wikipedia.org/wiki/Cron
[the Quartz Scheduler]: http://www.quartz-scheduler.org/documentation/quartz-2.3.0/tutorials/tutorial-lesson-06.html

此包的原始版本包含一个可选的 "秒" 字段，这使得它与这两种格式都不兼容。
现在，"标准" 格式是默认接受的格式，Quartz 格式是可选的。
