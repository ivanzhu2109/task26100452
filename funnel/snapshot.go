package funnel

import "sort"

// Snapshot 是从完整历史构建的只读分析快照。
// 构建后不再修改，可并发安全地查询。
type Snapshot struct {
	// histories 按商机分组的历史记录，每组内按时间戳升序。
	histories map[string][]Record
}

// NewSnapshot 从完整历史记录创建只读快照。
// 整体输入可以乱序，但同一商机的时间戳必须严格递增。
func NewSnapshot(records []Record) *Snapshot {
	histories := make(map[string][]Record)
	for _, r := range records {
		histories[r.OpportunityID] = append(histories[r.OpportunityID], r)
	}
	for _, h := range histories {
		sort.Slice(h, func(i, j int) bool { return h[i].TimestampMs < h[j].TimestampMs })
	}
	return &Snapshot{histories: histories}
}

// Query 按时间区间 [startMs, endMs)（左闭右开）查询五组阶段转化的指标。
// 返回结果按转化组顺序固定排列。
func (s *Snapshot) Query(startMs, endMs int64) []ConversionMetric {
	metrics := make([]ConversionMetric, len(conversionPairs))
	for i, p := range conversionPairs {
		metrics[i].From = p.From
		metrics[i].To = p.To
	}
	if startMs >= endMs {
		return metrics
	}
	for _, history := range s.histories {
		window := windowOf(history, startMs, endMs)
		if len(window) == 0 {
			continue
		}
		for i := range conversionPairs {
			den, num := countPair(window, conversionPairs[i])
			metrics[i].Denominator += den
			metrics[i].Numerator += num
		}
	}
	return metrics
}

// windowOf 返回按时间升序的历史中落在 [startMs, endMs) 内的子切片。
func windowOf(history []Record, startMs, endMs int64) []Record {
	lo := sort.Search(len(history), func(i int) bool { return history[i].TimestampMs >= startMs })
	hi := sort.Search(len(history), func(i int) bool { return history[i].TimestampMs >= endMs })
	return history[lo:hi]
}

// countPair 统计单个商机在窗口内对一组转化的贡献。
// 分母贡献：窗口内至少进入过一次来源阶段则为 1，否则为 0。
// 分子贡献：窗口内存在先进入来源、严格晚于它进入目标、且两条记录之间
// 没有赢单或输单记录，则为 1，否则为 0。两者均最多贡献 1。
func countPair(window []Record, p conversionPair) (denominator, numerator int) {
	seenFrom := false // 自上一次关闭（或窗口开始）以来是否进入过来源阶段
	for _, r := range window {
		if r.Stage == p.To && seenFrom {
			numerator = 1
		}
		if r.Stage == p.From {
			denominator = 1
			seenFrom = true
		}
		if r.Stage.IsClosed() {
			// 关闭结束一轮销售过程，关闭前的来源记录不能
			// 与关闭后的目标记录配对。
			seenFrom = false
		}
		if numerator == 1 && denominator == 1 {
			break
		}
	}
	return denominator, numerator
}
