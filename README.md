# rangemap

并发安全的 Go Range Map：维护 int64 半开区间 `[start, end)` 到非空字符串的映射。
纯标准库实现，无第三方依赖。

## 特性

- 内部始终规范化：区间按 start 升序、互不重叠、相邻同值自动合并
- `Assign` 覆盖指定范围，必要时切分原区间并与相邻同值区间合并
- `Delete` 删除指定范围，可把已有区间切成左右两段
- `sync.RWMutex` 保护，多 goroutine 可安全并发读写
- `Close` 后拒绝写操作（返回 `ErrClosed`），读操作仍可用，且可重复调用

## API

```go
m := rangemap.New()
err := m.Assign(0, 10, "a")   // [0,10) -> "a"
err  = m.Delete(3, 7)         // 剩下 [0,3) 和 [7,10)
v, ok := m.Get(8)             // "a", true
ivs := m.Snapshot()           // 有序独立副本 []rangemap.Interval
n   := m.Len()                // 规范化后的区间数量
m.Close()                     // 幂等；之后 Assign/Delete 返回 ErrClosed
```

参数校验：`start < end`（否则 `ErrInvalidRange`），`value` 非空（否则 `ErrEmptyValue`）。

## CLI 演示

演示覆盖、切分、相邻同值合并、删除以及 Close 后读取：

```
go run ./cmd/rangemap-demo
```

## 测试

```
go test -race ./...
```
