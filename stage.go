package funnel

// Stage 表示商机所处的销售阶段。
type Stage int

const (
	StageProspect    Stage = iota // 初探
	StageDiscovery                // 发现
	StageDemo                     // 演示
	StageProposal                 // 提案
	StageNegotiation              // 谈判
	StageWon                      // 赢单
	StageLost                     // 输单
)

// conversionPair 表示一组需要统计的阶段转化（来源阶段 -> 目标阶段）。
type conversionPair struct {
	from Stage
	to   Stage
}

// conversionPairs 返回需要统计的五组转化。
func conversionPairs() []conversionPair {
	return []conversionPair{
		{from: StageProspect, to: StageDiscovery},
		{from: StageDiscovery, to: StageDemo},
		{from: StageDemo, to: StageProposal},
		{from: StageProposal, to: StageNegotiation},
		{from: StageNegotiation, to: StageWon},
	}
}
