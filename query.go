package funnel

// ConversionMetric 表示一组阶段转化在时间窗口内的统计指标。
type ConversionMetric struct {
	From        Stage
	To          Stage
	Denominator int     // 窗口内至少进入过一次来源阶段的不同商机数
	Numerator   int     // 窗口内存在合格转化记录对的不同商机数
	Rate        float64 // Numerator / Denominator，分母为 0 时为 0
}

// ConversionMetrics 查询窗口 [startMs, endMs) 内五组转化的指标。
func (s *Snapshot) ConversionMetrics(startMs, endMs int64) []ConversionMetric {
	pairs := conversionPairs()
	metrics := make([]ConversionMetric, len(pairs))
	for i, p := range pairs {
		metrics[i] = ConversionMetric{From: p.from, To: p.to}
	}
	for _, records := range s.byOpportunity {
		window := recordsInWindow(records, startMs, endMs)
		if len(window) == 0 {
			continue
		}
		for i, p := range pairs {
			den, num := evaluatePair(window, p)
			metrics[i].Denominator += den
			metrics[i].Numerator += num
		}
	}
	for i := range metrics {
		if metrics[i].Denominator > 0 {
			metrics[i].Rate = float64(metrics[i].Numerator) / float64(metrics[i].Denominator)
		}
	}
	return metrics
}

// recordsInWindow 返回已按时间升序的记录中落在 [startMs, endMs) 内的部分。
func recordsInWindow(records []Record, startMs, endMs int64) []Record {
	lo := 0
	for lo < len(records) && records[lo].TimestampMs < startMs {
		lo++
	}
	hi := lo
	for hi < len(records) && records[hi].TimestampMs < endMs {
		hi++
	}
	return records[lo:hi]
}

// evaluatePair 计算单个商机在一组转化上对分母、分子的贡献（各为 0 或 1）。
// window 为该商机在窗口内按时间升序的记录。
func evaluatePair(window []Record, p conversionPair) (denominator, numerator int) {
	hasFrom := false
	for _, r := range window {
		if r.Stage == p.from {
			denominator = 1
			hasFrom = true
		}
		if r.Stage == p.to && hasFrom {
			numerator = 1
		}
		if r.Stage == StageWon || r.Stage == StageLost {
			// 赢单/输单结束一轮销售过程，关闭前的来源记录不能跨关闭配对。
			hasFrom = false
		}
	}
	return denominator, numerator
}
