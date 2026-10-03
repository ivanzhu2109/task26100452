package funnel

import "testing"

func metricFor(t *testing.T, metrics []ConversionMetric, from, to Stage) ConversionMetric {
	t.Helper()
	for _, m := range metrics {
		if m.From == from && m.To == to {
			return m
		}
	}
	t.Fatalf("metric %v->%v not found", from, to)
	return ConversionMetric{}
}

func TestConversionMetricsReturnsFivePairs(t *testing.T) {
	snap := NewSnapshot(nil)
	metrics := snap.ConversionMetrics(0, 100)
	if len(metrics) != 5 {
		t.Fatalf("expected 5 metrics, got %d", len(metrics))
	}
	for _, m := range metrics {
		if m.Denominator != 0 || m.Numerator != 0 || m.Rate != 0 {
			t.Fatalf("expected zero metric, got %+v", m)
		}
	}
}

// 同一商机依次在 10、20、30、40 进入初探、发现、初探、发现。
func TestRepeatedEntriesSameOpportunity(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "A", TimestampMs: 10, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageDiscovery},
		{OpportunityID: "A", TimestampMs: 30, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 40, Stage: StageDiscovery},
	})

	m := metricFor(t, snap.ConversionMetrics(10, 41), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 1 {
		t.Fatalf("window [10,41): expected den=1 num=1, got den=%d num=%d", m.Denominator, m.Numerator)
	}
	if m.Rate != 1 {
		t.Fatalf("window [10,41): expected rate=1, got %v", m.Rate)
	}

	m = metricFor(t, snap.ConversionMetrics(21, 40), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("window [21,40): expected den=1 num=0, got den=%d num=%d", m.Denominator, m.Numerator)
	}
	if m.Rate != 0 {
		t.Fatalf("window [21,40): expected rate=0, got %v", m.Rate)
	}
}

// 关闭后重开：初探、输单、发现不构成转化；再次初探再发现才构成；
// 覆盖整段的窗口仍只计入一个商机。
func TestReopenAfterClose(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "A", TimestampMs: 10, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageLost},
		{OpportunityID: "A", TimestampMs: 30, Stage: StageDiscovery},
		{OpportunityID: "A", TimestampMs: 40, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 50, Stage: StageDiscovery},
	})

	// 只覆盖关闭前的初探与关闭后的发现：不能跨关闭配对。
	m := metricFor(t, snap.ConversionMetrics(10, 31), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("window [10,31): expected den=1 num=0, got den=%d num=%d", m.Denominator, m.Numerator)
	}

	// 覆盖整段：重开后的初探->发现构成一次合格转化，同一商机只计一次。
	m = metricFor(t, snap.ConversionMetrics(0, 100), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 1 {
		t.Fatalf("window [0,100): expected den=1 num=1, got den=%d num=%d", m.Denominator, m.Numerator)
	}
}

// 赢单同样结束一轮销售过程：谈判、赢单、初探、发现中，
// 赢单前的记录不能与赢单后的记录配对。
func TestWonEndsSalesCycle(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "A", TimestampMs: 10, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageWon},
		{OpportunityID: "A", TimestampMs: 30, Stage: StageDiscovery},
	})
	m := metricFor(t, snap.ConversionMetrics(0, 100), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("expected den=1 num=0, got den=%d num=%d", m.Denominator, m.Numerator)
	}
}

// 跳阶段不补记被跳过的阶段：初探之后直接进入演示，
// 初探->发现 与 发现->演示 均不产生转化，发现阶段分母为 0。
func TestSkipStages(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "A", TimestampMs: 10, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageDemo},
	})
	metrics := snap.ConversionMetrics(0, 100)

	m := metricFor(t, metrics, StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("prospect->discovery: expected den=1 num=0, got den=%d num=%d", m.Denominator, m.Numerator)
	}
	m = metricFor(t, metrics, StageDiscovery, StageDemo)
	if m.Denominator != 0 || m.Numerator != 0 || m.Rate != 0 {
		t.Fatalf("discovery->demo: expected den=0 num=0 rate=0, got %+v", m)
	}
	m = metricFor(t, metrics, StageDemo, StageProposal)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("demo->proposal: expected den=1 num=0, got den=%d num=%d", m.Denominator, m.Numerator)
	}
}

// 窗口外的记录不能补足窗口内缺失的来源或目标；窗口左闭右开。
func TestWindowIsLeftClosedRightOpen(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "A", TimestampMs: 5, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 10, Stage: StageDiscovery},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 30, Stage: StageDiscovery},
	})

	// 来源在窗口外（5 < 10），目标在窗口内：不构成转化，分母也不计。
	m := metricFor(t, snap.ConversionMetrics(10, 20), StageProspect, StageDiscovery)
	if m.Denominator != 0 || m.Numerator != 0 {
		t.Fatalf("window [10,20): expected den=0 num=0, got den=%d num=%d", m.Denominator, m.Numerator)
	}

	// 右开：30 不在 [10,30) 内，窗口内只有目标没有来源。
	m = metricFor(t, snap.ConversionMetrics(10, 30), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("window [10,30): expected den=1 num=0, got den=%d num=%d", m.Denominator, m.Numerator)
	}

	// 左闭：20 在 [20,31) 内，与 30 的发现构成转化。
	m = metricFor(t, snap.ConversionMetrics(20, 31), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 1 {
		t.Fatalf("window [20,31): expected den=1 num=1, got den=%d num=%d", m.Denominator, m.Numerator)
	}
}

// 乱序输入与多商机聚合；转化率按不同商机计算。
func TestUnorderedInputAndMultipleOpportunities(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "B", TimestampMs: 12, Stage: StageDiscovery},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageDiscovery},
		{OpportunityID: "B", TimestampMs: 10, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 10, Stage: StageProspect},
		{OpportunityID: "C", TimestampMs: 15, Stage: StageProspect},
	})
	m := metricFor(t, snap.ConversionMetrics(0, 100), StageProspect, StageDiscovery)
	if m.Denominator != 3 || m.Numerator != 2 {
		t.Fatalf("expected den=3 num=2, got den=%d num=%d", m.Denominator, m.Numerator)
	}
	want := 2.0 / 3.0
	if m.Rate != want {
		t.Fatalf("expected rate=%v, got %v", want, m.Rate)
	}
}

// 谈判->赢单：目标本身是赢单时仍可构成合格转化。
func TestNegotiationToWon(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "A", TimestampMs: 10, Stage: StageNegotiation},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageWon},
		{OpportunityID: "B", TimestampMs: 10, Stage: StageNegotiation},
		{OpportunityID: "B", TimestampMs: 20, Stage: StageLost},
	})
	m := metricFor(t, snap.ConversionMetrics(0, 100), StageNegotiation, StageWon)
	if m.Denominator != 2 || m.Numerator != 1 {
		t.Fatalf("expected den=2 num=1, got den=%d num=%d", m.Denominator, m.Numerator)
	}
	if m.Rate != 0.5 {
		t.Fatalf("expected rate=0.5, got %v", m.Rate)
	}
}

// 中间经过其他非关闭阶段不影响配对。
func TestIntermediateNonClosingStages(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "A", TimestampMs: 10, Stage: StageProspect},
		{OpportunityID: "A", TimestampMs: 20, Stage: StageDemo},
		{OpportunityID: "A", TimestampMs: 30, Stage: StageProposal},
		{OpportunityID: "A", TimestampMs: 40, Stage: StageDiscovery},
	})
	m := metricFor(t, snap.ConversionMetrics(0, 100), StageProspect, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 1 {
		t.Fatalf("expected den=1 num=1, got den=%d num=%d", m.Denominator, m.Numerator)
	}
}
