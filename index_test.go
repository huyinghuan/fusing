package fusing

import (
  "fmt"
  "math/rand"
  "os"
  "testing"
  "time"
)

func TestMain(m *testing.M){
  Log = func(s string) {
    fmt.Println(s)
  }
  os.Exit(m.Run())
}

// 测试 QPS 2个资源
func TestGo(t *testing.T) {
  resourceCount := 2
  Init(FlowRule{
    ActiveOnQPS:   50,
    Period:        5 * time.Second,
    DegradeRate:   10,
    FastRecover:   70,
    PeriodRecover: 10,
    MinFlow:       10,
  })

  for i:=0;i < resourceCount; i++{
    AddResource(fmt.Sprintf("id-%d", i))
  }

  stop := make(chan interface{})

  errRate := 30

  go func() {
    // 25秒后，错误率降到5，模拟服务恢复
    time.AfterFunc(25 * time.Second, func() {
      errRate = 5
    })
    // 1分钟后，服务结束【这个是应该请求全部过去率】
    time.AfterFunc(time.Minute, func() {
      stop <- nil
    })
  }()

  for i := 0; i< 10; i++{
    go func() {
      rand.Seed(time.Now().UnixNano())
      for {
        sourceId := fmt.Sprintf("id-%d", rand.Intn(resourceCount) )
        if Pass(sourceId){
            IncrementRequest(sourceId)
            // 错误率设置为 30
            if rand.Intn(100) < errRate{
              IncrementError(sourceId)
            }
        }
        // QPS 模拟
        time.Sleep(1 * time.Millisecond)
      }
    }()
  }
  <-stop
}

// 测试 ActiveOnQPS 配置
func TestActiveConf(t *testing.T) {

  resourceCount := 3
  Init(FlowRule{
    ActiveOnQPS:   100, // qps小于100的时候，不触发熔断
    Period:        10 * time.Second,
    DegradeRate:   10,
    FastRecover:   70,
    PeriodRecover: 10,
    MinFlow:       10,
  })

  for i:=0;i < resourceCount; i++{
    AddResource(fmt.Sprintf("id-%d", i))
  }

  stop := make(chan interface{})
  errRate := 30

  go func() {
    // 25秒后，错误率降到5，模拟服务恢复
    time.AfterFunc(25 * time.Second, func() {
      errRate = 5
    })
    // 1分钟后，服务结束【这个是应该请求全部过去率】
    time.AfterFunc(time.Minute, func() {
      stop <- nil
    })
  }()
  for i := 0; i< 10; i++{
    go func() {
      rand.Seed(time.Now().UnixNano())
      for {
        sourceId := fmt.Sprintf("id-%d", rand.Intn(resourceCount) )
        if Pass(sourceId){
          IncrementRequest(sourceId)
          // 错误率设置为 30
          if rand.Intn(100) < errRate{
            IncrementError(sourceId)
          }

        }
        // QPS 模拟
        time.Sleep(time.Duration(rand.Uint64() % 70 + 1)*time.Millisecond)
      }
    }()
  }
  <-stop


}

// 测试资源读写并发
func TestResourceRW(t *testing.T){
  resourceCount := 3
  Init(FlowRule{
    ActiveOnQPS:   100, // qps小于100的时候，不触发熔断
    Period:        10 * time.Second,
    DegradeRate:   10,
    FastRecover:   70,
    PeriodRecover: 10,
    MinFlow:       10,
  })

  for i:=0;i < resourceCount; i++{
    AddResource(fmt.Sprintf("id-%d", i))
  }

  stop := make(chan interface{})
  errRate := 30

  go func() {
    // 25秒后，错误率降到5，模拟服务恢复
    time.AfterFunc(25 * time.Second, func() {
      errRate = 5
    })
    // 1分钟后，服务结束【这个是应该请求全部过去率】
    time.AfterFunc(90 * time.Second, func() {
      stop <- nil
    })
    // 5秒后，资源开始添加
    time.AfterFunc(5 * time.Second, func() {
      queue := make(chan int, 10)
      for j:=0;j<5;j++{
        go func() {
          for{
            id := <- queue
            AddResource(fmt.Sprintf("id-%d", id))
          }
        }()
      }

      for i := 5; i < 100;i++{
        queue <- i
      }
    })
  }()
  for i := 0; i< 10; i++{
    go func() {
      rand.Seed(time.Now().UnixNano())
      for {
        sourceId := fmt.Sprintf("id-%d", rand.Intn(100) )
        if Pass(sourceId){
          IncrementRequest(sourceId)
          // 错误率设置为 30
          if rand.Intn(100) < errRate{
            IncrementError(sourceId)
          }

        }
        // QPS 模拟
        time.Sleep( time.Duration(rand.Uint64() % 70 + 1) *time.Microsecond)
      }
    }()
  }
  <-stop

}

// 性能测试
func BenchmarkAddResource(b *testing.B) {
  b.StopTimer()
  resourceCount := 3
  Init(FlowRule{
    ActiveOnQPS:   100, // qps小于100的时候，不触发熔断
    Period:        10 * time.Second,
    DegradeRate:   10,
    FastRecover:   70,
    PeriodRecover: 10,
    MinFlow:       10,
  })
  b.StartTimer()

  for i := 0; i < b.N; i++ {
    sourceId := fmt.Sprintf("id-%d", rand.Intn(resourceCount) )
    if Pass(sourceId){
      IncrementRequest(sourceId)
      // 错误率设置为 30
      if rand.Intn(100) < 30{
        IncrementError(sourceId)
      }
    }
  }


}
