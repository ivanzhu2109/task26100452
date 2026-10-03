// Package funnel 提供商机阶段转化分析能力。
//
// 阶段历史与分析快照均保存在进程内存中，由上层直接调用。
// 快照构建后只读，不再修改。
package funnel

// Stage 表示商机所处的销售阶段。
type Stage int

const (
	StageInitial     Stage = iota // 初探
	StageDiscovery                // 发现
	StageDemo                     // 演示
	StageProposal                 // 提案
	StageNegotiation              // 谈判
	StageWon                      // 赢单
	StageLost                     // 输单
)

// IsClosed 报告该阶段是否为关闭阶段（赢单或输单）。
// 关闭阶段会结束一轮销售过程。
func (s Stage) IsClosed() bool {
	return s == StageWon || s == StageLost
}

// Record 表示一条阶段历史记录：某商机在某时刻进入了某阶段。
// 每条记录都代表一次进入，包括首次记录；同一商机可以多次进入同一阶段。
type Record struct {
	OpportunityID string // 商机标识
	TimestampMs   int64  // UTC 整数毫秒时间戳，同一商机内严格递增
	Stage         Stage  // 进入的阶段
}

// conversionPair 描述一组需要统计的阶段转化（来源阶段 -> 目标阶段）。
type conversionPair struct {
	From Stage
	To   Stage
}

// conversionPairs 为需要统计的五组转化：初探->发现、发现->演示、
// 演示->提案、提案->谈判、谈判->赢单。跳阶段不补记被跳过的阶段。
var conversionPairs = []conversionPair{
	{StageInitial, StageDiscovery},
	{StageDiscovery, StageDemo},
	{StageDemo, StageProposal},
	{StageProposal, StageNegotiation},
	{StageNegotiation, StageWon},
}

// ConversionMetric 是一组阶段转化在查询窗口内的统计结果。
type ConversionMetric struct {
	From Stage // 来源阶段
	To   Stage // 目标阶段

	Denominator int // 窗口内至少进入过一次来源阶段的不同商机数
	Numerator   int // 窗口内存在一对合格转化记录的不同商机数
}

// Rate 返回转化率（分子/分母），分母为 0 时返回 0。
func (m ConversionMetric) Rate() float64 {
	if m.Denominator == 0 {
		return 0
	}
	return float64(m.Numerator) / float64(m.Denominator)
}
