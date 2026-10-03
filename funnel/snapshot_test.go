package funnel

import "testing"

// metricFor 在查询结果中查找指定转化组的指标。
func metricFor(t *testing.T, metrics []ConversionMetric, from, to Stage) ConversionMetric {
	t.Helper()
	for _, m := range metrics {
		if m.From == from && m.To == to {
			return m
		}
	}
	t.Fatalf("缺少转化组 %d -> %d 的指标", from, to)
	return ConversionMetric{}
}

// TestRepeatedEntrySameOpportunity 同一商机多次进入同一阶段，
// 分母与分子都最多贡献 1。
//
// 商机依次在时间 10、20、30、40 进入初探、发现、初探、发现：
//   - 窗口 [10,41)：初探->发现 分母=1，分子=1
//   - 窗口 [21,40)：初探->发现 分母=1，分子=0
func TestRepeatedEntrySameOpportunity(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "opp-1", TimestampMs: 10, Stage: StageInitial},
		{OpportunityID: "opp-1", TimestampMs: 20, Stage: StageDiscovery},
		{OpportunityID: "opp-1", TimestampMs: 30, Stage: StageInitial},
		{OpportunityID: "opp-1", TimestampMs: 40, Stage: StageDiscovery},
	})

	m := metricFor(t, snap.Query(10, 41), StageInitial, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 1 {
		t.Fatalf("窗口 [10,41): 期望分母=1 分子=1，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}
	if got := m.Rate(); got != 1 {
		t.Fatalf("窗口 [10,41): 期望转化率=1，实际=%v", got)
	}

	m = metricFor(t, snap.Query(21, 40), StageInitial, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("窗口 [21,40): 期望分母=1 分子=0，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}
}

// TestReopenAfterClose 关闭（赢单/输单）结束一轮销售过程：
// 初探、输单、发现不能构成初探->发现的转化；
// 之后再次初探再发现才可以，且覆盖整段的窗口仍只计入一个商机。
func TestReopenAfterClose(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "opp-1", TimestampMs: 10, Stage: StageInitial},
		{OpportunityID: "opp-1", TimestampMs: 20, Stage: StageLost},
		{OpportunityID: "opp-1", TimestampMs: 30, Stage: StageDiscovery},
		{OpportunityID: "opp-1", TimestampMs: 40, Stage: StageInitial},
		{OpportunityID: "opp-1", TimestampMs: 50, Stage: StageDiscovery},
	})

	// 只覆盖第一轮：初探、输单、发现，不能构成转化。
	m := metricFor(t, snap.Query(10, 31), StageInitial, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("窗口 [10,31): 期望分母=1 分子=0，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}

	// 覆盖整段：第二轮构成一次合格转化，但同一商机只计入一次。
	m = metricFor(t, snap.Query(0, 100), StageInitial, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 1 {
		t.Fatalf("窗口 [0,100): 期望分母=1 分子=1，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}
}

// TestSkipStage 跳阶段不补记被跳过的阶段：
// 初探之后直接进入演示，不会产生“发现”阶段的进入记录。
func TestSkipStage(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "opp-1", TimestampMs: 10, Stage: StageInitial},
		{OpportunityID: "opp-1", TimestampMs: 20, Stage: StageDemo},
	})

	m := metricFor(t, snap.Query(0, 100), StageInitial, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("初探->发现: 期望分母=1 分子=0，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}

	m = metricFor(t, snap.Query(0, 100), StageDiscovery, StageDemo)
	if m.Denominator != 0 || m.Numerator != 0 {
		t.Fatalf("发现->演示: 期望分母=0 分子=0，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}
	if got := m.Rate(); got != 0 {
		t.Fatalf("发现->演示: 分母为 0 时期望转化率=0，实际=%v", got)
	}
}

// TestNegotiationToWon 目标本身是赢单时仍可构成合格转化。
func TestNegotiationToWon(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "opp-1", TimestampMs: 10, Stage: StageNegotiation},
		{OpportunityID: "opp-1", TimestampMs: 20, Stage: StageWon},
		{OpportunityID: "opp-2", TimestampMs: 10, Stage: StageNegotiation},
		{OpportunityID: "opp-2", TimestampMs: 20, Stage: StageLost},
		{OpportunityID: "opp-2", TimestampMs: 30, Stage: StageWon},
	})

	m := metricFor(t, snap.Query(0, 100), StageNegotiation, StageWon)
	if m.Denominator != 2 || m.Numerator != 1 {
		t.Fatalf("谈判->赢单: 期望分母=2 分子=1，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}
	if got := m.Rate(); got != 0.5 {
		t.Fatalf("谈判->赢单: 期望转化率=0.5，实际=%v", got)
	}
}

// TestWindowBoundaries 窗口左闭右开，窗口外的记录不能补足
// 窗口内缺失的来源或目标。
func TestWindowBoundaries(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "opp-1", TimestampMs: 10, Stage: StageInitial},
		{OpportunityID: "opp-1", TimestampMs: 20, Stage: StageDiscovery},
	})

	// 来源在窗口左边界上（左闭），目标在窗口内：构成转化。
	m := metricFor(t, snap.Query(10, 21), StageInitial, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 1 {
		t.Fatalf("窗口 [10,21): 期望分母=1 分子=1，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}

	// 目标在窗口右边界上（右开），被排除：只有分母。
	m = metricFor(t, snap.Query(10, 20), StageInitial, StageDiscovery)
	if m.Denominator != 1 || m.Numerator != 0 {
		t.Fatalf("窗口 [10,20): 期望分母=1 分子=0，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}

	// 来源在窗口开始前已进入、窗口内未再次进入：不计入分母，
	// 窗口外的来源也不能与窗口内的目标配对。
	m = metricFor(t, snap.Query(15, 25), StageInitial, StageDiscovery)
	if m.Denominator != 0 || m.Numerator != 0 {
		t.Fatalf("窗口 [15,25): 期望分母=0 分子=0，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}
}

// TestUnorderedInput 整体输入可以乱序，快照构建时按商机分组排序。
func TestUnorderedInput(t *testing.T) {
	snap := NewSnapshot([]Record{
		{OpportunityID: "opp-2", TimestampMs: 20, Stage: StageDiscovery},
		{OpportunityID: "opp-1", TimestampMs: 30, Stage: StageDiscovery},
		{OpportunityID: "opp-1", TimestampMs: 10, Stage: StageInitial},
		{OpportunityID: "opp-2", TimestampMs: 10, Stage: StageInitial},
	})

	m := metricFor(t, snap.Query(0, 100), StageInitial, StageDiscovery)
	if m.Denominator != 2 || m.Numerator != 2 {
		t.Fatalf("期望分母=2 分子=2，实际分母=%d 分子=%d", m.Denominator, m.Numerator)
	}
}

// TestEmptyAndInvalidWindow 空快照与非法窗口返回全零指标。
func TestEmptyAndInvalidWindow(t *testing.T) {
	snap := NewSnapshot(nil)
	for _, m := range snap.Query(0, 100) {
		if m.Denominator != 0 || m.Numerator != 0 || m.Rate() != 0 {
			t.Fatalf("空快照: 期望全零，实际 %+v", m)
		}
	}

	snap = NewSnapshot([]Record{
		{OpportunityID: "opp-1", TimestampMs: 10, Stage: StageInitial},
		{OpportunityID: "opp-1", TimestampMs: 20, Stage: StageDiscovery},
	})
	for _, m := range snap.Query(50, 50) {
		if m.Denominator != 0 || m.Numerator != 0 {
			t.Fatalf("空窗口: 期望全零，实际 %+v", m)
		}
	}
}
