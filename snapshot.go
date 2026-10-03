package funnel

import "sort"

// Record 表示一条阶段历史记录：某个商机在某个 UTC 毫秒时间戳进入了某个阶段。
type Record struct {
	OpportunityID string
	TimestampMs   int64
	Stage         Stage
}

// Snapshot 是从完整历史构建的只读分析快照，构建后不再修改。
type Snapshot struct {
	// byOpportunity 按商机分组的历史记录，每个商机内部按时间戳升序。
	byOpportunity map[string][]Record
}

// NewSnapshot 从完整历史记录创建只读快照。输入可以整体乱序。
func NewSnapshot(records []Record) *Snapshot {
	grouped := make(map[string][]Record)
	for _, r := range records {
		grouped[r.OpportunityID] = append(grouped[r.OpportunityID], r)
	}
	for _, rs := range grouped {
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].TimestampMs < rs[j].TimestampMs
		})
	}
	return &Snapshot{byOpportunity: grouped}
}
