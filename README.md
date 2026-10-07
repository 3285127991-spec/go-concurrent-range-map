# go-concurrent-range-map

并发安全的 Range Map：把 int64 半开区间 `[start, end)` 映射到非空字符串 value。
纯 Go 标准库实现，无第三方依赖。

## 特性

- 区间始终按 start 升序、互不重叠
- `Assign` 覆盖指定范围，必要时切分原区间，相邻同值区间自动合并
- `Delete` 删除指定范围，可把已有区间切成左右两段
- `Get` / `Snapshot` / `Len` 与写操作可安全并发（`sync.RWMutex` 保护）
- `Close` 后拒绝写入，读取仍可用，可重复调用

## API

```go
m := new(rangemap.RangeMap)
err := m.Assign(0, 10, "a")   // [0,10) -> "a"
err  = m.Delete(3, 7)         // 切分为 [0,3) 和 [7,10)
v, ok := m.Get(8)             // "a", true
snap := m.Snapshot()          // 有序独立副本 []rangemap.Interval
n    := m.Len()               // 规范化后的区间数量
m.Close()                     // 幂等；之后 Assign/Delete 返回 ErrClosed
```

错误：`ErrInvalidRange`（start >= end）、`ErrEmptyValue`（空 value）、`ErrClosed`（关闭后写入）。

## 目录结构

- `rangemap.go` — 库实现
- `rangemap_test.go` — 自动化测试
- `cmd/rangemap-demo/` — 演示 CLI（覆盖、切分、合并、删除、Close 后读取）

## 运行

```sh
go test -race ./...
go run ./cmd/rangemap-demo
```
