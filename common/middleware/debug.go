package common

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/middleware"
)

type DebugMiddleware struct {
	middleware.Abstract
}

func (self DebugMiddleware) Process(ctx *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	slog.Info("runtime",
		"url", ctx.Request.URL,
		"goroutine", fmt.Sprintf("%d", runtime.NumGoroutine()),
		"client", ws.GetCollect().Total(),
		"progress", ws.GetCollect().ProgressTotal(),

		// 内存（带单位）
		"alloc", fmt.Sprintf("%dMB", m.Alloc>>20), // 业务堆内存：持续增长 → 泄漏
		"sys", fmt.Sprintf("%dMB", m.Sys>>20), // 总内存（≈htop）：容器 OOM 主因
		"heapIdle", fmt.Sprintf("%dMB", m.HeapIdle>>20), // 堆空闲内存
		"heapRelease", fmt.Sprintf("%dMB", m.HeapReleased>>20), // 已归还 OS 的内存
		"heapKeep", fmt.Sprintf("%dMB", (m.HeapIdle-m.HeapReleased)>>20), // = idle - rel，保留未释放；>50MB 可考虑 GODEBUG=madvdontneed=1

		// 栈与 GC（带单位）
		"stk", fmt.Sprintf("%dMB", m.StackInuse>>20), // goroutine 栈内存：配合 goroutine 判断是否异常
		"gc", fmt.Sprintf("%d", m.NumGC), // GC 次数：单位时间激增 → 分配过快
		"gcpu", fmt.Sprintf("%d%%", int(m.GCCPUFraction*100)), // GC CPU%：<10% 正常，>20% 需优化
		"stw", fmt.Sprintf("%dms", m.PauseTotalNs/1e6), // STW 总暂停：对延迟敏感服务需 < 50ms/分钟
	)
	ctx.Next()
}
