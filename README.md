# Fusing

Fusing 一个延迟和容错库，旨在隔离对远程系统，服务和第三方库的访问点，停止级联故障.

Note: copy from xionghu@mgtv.com, update some code

## Getting Start

获取：

```
go get -u github.com/huyinghuan/fusing
```

使用

1. 初始化规则

```
fusing.Init(FlowRule{
    ActiveOnQPS:   50,
    Period:        5 * time.Second,
    DegradeRate:   10,
    FastRecover:   70,
    PeriodRecover: 10,
    MinFlow:       10,
  }, func(s string) {

 })
```

2. 定义资源

```
fusing.AddResource("id-xxxx")
```


3. 业务中使用

```
sourceId:="id-xxxx"
// 是否执行资源【是否已经熔断】
if fusing.Pass(sourceId){
    fusing.IncrementRequest(sourceId)
    // TODO  业务逻辑在这里
    // 请求第三方接口
    r, err := http.Get("xxxxx")
    if err != nil{
        // 第三方出现错误的情况下，增加一个错误请求
        fusing.IncrementError(sourceId)
    }
}
```

更多:
[index_test.go](./index_test.go)

