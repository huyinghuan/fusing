// Package fusing 一个延迟和容错库，旨在隔离对远程系统，服务和第三方库的访问点，停止级联故障.
//
// 快速开始:
//  fusing.Init(FlowRule{
//    ActiveOnQPS:   50,
//    Period:        5 * time.Second,
//    DegradeRate:   10,
//    FastRecover:   70,
//    PeriodRecover: 10,
//    MinFlow:       10,
//  })
//  // 记录日志输出。可以不定义
//  fusing.Log = func(s string){fmt.Println(s)}
//  fusing.AddResource("id-xxxx")
//  sourceId:="id-xxxx"
//  // 是否执行资源【是否已经熔断】
//  if fusing.Pass(sourceId){
//    fusing.IncrementRequest(sourceId)
//     // TODO  业务逻辑在这里
//      // 请求第三方接口
//    r, err := http.Get("xxxxx")
//    if err != nil{
//      // 第三方出现错误的情况下，增加一个错误请求
//      fusing.IncrementError(sourceId)
//    }
//  }
package fusing

import (
  "log"
  "math/rand"
  "strconv"
  "strings"
  "sync"
  "time"
)

// FlowRule 流量规则
type FlowRule struct {
  // ActiveOnQPS QPS达到某个数值时，激活依赖服务的流量控制规则
  ActiveOnQPS int
  // Period 计算周期
  Period time.Duration
  // DegradeRate 当请求错误比率达到DegradeRate值后，开始对 依赖服务的流量控制
  DegradeRate int 
  // FastRecover 当请求错误比率下降到DegradeRate 后，开始逐步解除对依赖服务的流量控制
  // 快速恢复 请求的通过率，  在通过率达到 FastRecover 之前， 每个计算周期内，通过率翻倍
  FastRecover        int
  // PeriodRecover 当通过率 达到  快速恢复通过率 之后，通过率每次增加 PeriodRecover 直到 100%
  PeriodRecover int
  // MinFlow 请求的最小流量 【请求的通过率】
  MinFlow int
}

var (
  flowRule  FlowRule
  flowRateMap = map[string]*resource{}
  mapLocker   = new(sync.RWMutex)
  // Log 记录日志输出
  Log func(string)
)

// 资源
type resource struct {
  ID          string
  reqSum      int // 计算周期内 资源的请求数量
  errorSum    int  //  计算周期内 资源的请求错误的数量
  flowRate    int //  当前通过率
  errorRate int // 请求错误的率
  qps         int //
  blocked     int // 放弃的请求
}

func updateQPS() {
  mapLocker.RLock()
  for _, v := range flowRateMap {
    Log(strings.Join([]string{
      time.Now().Format("2006-01-02 15:04:05"),
      v.ID,
      strconv.Itoa(v.qps - v.blocked),
      strconv.Itoa(v.blocked),
      strconv.Itoa(v.qps),
    }, "|"))
    v.qps = 0
    v.blocked = 0
  }
  mapLocker.RUnlock()
}

// 初始化流量降级服务
func Init(rule FlowRule) {
  flowRule = rule
  if flowRule.Period < time.Second{
    log.Fatal("Calculation period cannot not be less than 1s")
  }
  flowTimer := time.NewTicker(flowRule.Period)
  qpsTimer := time.NewTicker(time.Second)
  go func() {
    for {
      select {
      case _ = <-flowTimer.C:
        updateFlowRate()
      case _ = <-qpsTimer.C:
        updateQPS()
      }
    }
  }()
}

// 增加一个资源
func AddResource(id string) bool {
  mapLocker.Lock()
  defer mapLocker.Unlock()
  _, flag := flowRateMap[id]
  if !flag {
    flowRateMap[id] = &resource{ID: id, flowRate: 100}
    return true
  } else {
    return false
  }
}

// 周期性计算所有资源流量
func updateFlowRate() {
  mapLocker.RLock()
  for _, v := range flowRateMap {
    calculateFlowRate(v)
  }
  mapLocker.RUnlock()
}

// 周期性计算某个资源流量
func calculateFlowRate(item *resource) {
  if item.reqSum == 0 {
    item.errorRate = 0
    item.errorSum = 0
    return
  }
  item.errorRate = int(100 * float32(item.errorSum) / float32(item.reqSum))
  // 某段时间内超时比例过大时，把允许通过的流量设置为当前的一半 【默认为100，降一半为50】
  // 如果继续超时，继续降一半，直到允许通过的最小比例。【即熔断后，还是允许一定比率的流量去访问接口，已达到自动恢复】
  if item.errorRate > flowRule.DegradeRate {
    item.flowRate /= 2
    if item.flowRate < flowRule.MinFlow {
      item.flowRate = flowRule.MinFlow
    }
  } else {
    // 没有超时了，如果通过率小于 flowConfig.FastRecover，
    // 那么将通过率翻倍，翻倍后最大值为flowConfig.FastRecover
    if item.flowRate < flowRule.FastRecover {
      item.flowRate *= 2
      if item.flowRate > flowRule.FastRecover {
        item.flowRate = flowRule.FastRecover
      }
    } else if item.flowRate <= (100 - flowRule.PeriodRecover) {
      // 没有超时了 ，在某个计时段，如果通过率达到某个阀值，那么以后的每个计时段，通过率增加 incrementRatio
      item.flowRate += flowRule.PeriodRecover
    }
  }
  if item.flowRate > 100 {
    item.flowRate = 100
  }
  item.reqSum = 0
  item.errorSum = 0
}

// IncrementRequest 增加一个资源的请求计数器
func IncrementRequest(resourceId string) bool {
  mapLocker.RLock()
  item, flag := flowRateMap[resourceId]
  mapLocker.RUnlock()
  if flag == false {
    return false
  }
  item.reqSum += 1
  return true
}

// IncrementError 增加一个资源的错误请求计数器
func IncrementError(resourceId string) bool {
  mapLocker.RLock()
  item, flag := flowRateMap[resourceId]
  mapLocker.RUnlock()
  if flag == false {
    return false
  }
  item.errorSum += 1
  return true
}

// Pass 是否允许资源执行
func Pass(resourceId string) bool {
  mapLocker.RLock()
  item, flag := flowRateMap[resourceId]

  mapLocker.RUnlock()
  if flag == false {
    return true
  }
  item.qps += 1
  if item.qps < flowRule.ActiveOnQPS {
    return true
  }
  rand := rand.Intn(100)
  if rand < item.flowRate {
    return true
  } else {
    item.blocked += 1
    return false
  }
}
