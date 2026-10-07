package store

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

// Memory 是并发安全的内存仓储。所有写入都在写锁内完成状态校验，
// 读取则通过 Snapshot 得到一致视图。
type Memory struct {
	mu sync.RWMutex

	apiaries    map[string]domain.Apiary
	hives       map[string]domain.Hive
	inspections map[string][]domain.Inspection
	queens      map[string][]domain.Queen
	handovers   map[string][]domain.KeeperHandover

	inspectionSeq int
	queenSeq      int
	handoverSeq   int
}

func NewMemory() *Memory {
	return &Memory{
		apiaries:    make(map[string]domain.Apiary),
		hives:       make(map[string]domain.Hive),
		inspections: make(map[string][]domain.Inspection),
		queens:      make(map[string][]domain.Queen),
		handovers:   make(map[string][]domain.KeeperHandover),
	}
}

func (m *Memory) CreateApiary(apiary domain.Apiary) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apiaries[apiary.ID]; ok {
		return domain.Conflict("蜂场 %q 已存在", apiary.ID)
	}
	m.apiaries[apiary.ID] = apiary
	return nil
}

func (m *Memory) GetApiary(id string) (domain.Apiary, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	apiary, ok := m.apiaries[id]
	return apiary, ok
}

// HandoverKeeper 在同一把写锁内完成当前值守人核对、交接记录写入与蜂场值守人切换。
// 协调人声明的 fromKeeper 必须与当前值守人一致：并发交接中只有第一个提交者能匹配成功，
// 其余提交者看到已切换的值守人而得到冲突，保证值守人切换是一次串行的线性化提交。
// 交接只改蜂场的当前值守人，不回写任何蜂箱与巡检的历史归属。
func (m *Memory) HandoverKeeper(handover domain.KeeperHandover) (domain.KeeperHandover, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	apiary, ok := m.apiaries[handover.ApiaryID]
	if !ok {
		return domain.KeeperHandover{}, domain.NotFound("蜂场", handover.ApiaryID)
	}
	if apiary.Keeper != handover.FromKeeper {
		return domain.KeeperHandover{}, domain.Conflict(
			"蜂场 %q 当前值守人为 %q，与声明的 fromKeeper %q 不一致，可能已发生交接",
			handover.ApiaryID, apiary.Keeper, handover.FromKeeper)
	}
	m.handoverSeq++
	handover.ID = fmt.Sprintf("handover-%06d", m.handoverSeq)
	m.handovers[handover.ApiaryID] = append(m.handovers[handover.ApiaryID], handover)
	apiary.Keeper = handover.ToKeeper
	m.apiaries[handover.ApiaryID] = apiary
	return handover, nil
}

// ListHandovers 返回蜂场的交接记录，按交接时刻升序；时刻相同时按编号升序，顺序稳定。
func (m *Memory) ListHandovers(apiaryID string) ([]domain.KeeperHandover, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.apiaries[apiaryID]; !ok {
		return nil, domain.NotFound("蜂场", apiaryID)
	}
	items := m.handovers[apiaryID]
	out := append([]domain.KeeperHandover{}, items...)
	sort.Slice(out, func(i, j int) bool {
		if !out[i].HandedOverAt.Equal(out[j].HandedOverAt) {
			return out[i].HandedOverAt.Before(out[j].HandedOverAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *Memory) CreateHive(hive domain.Hive) (domain.Hive, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	apiary, ok := m.apiaries[hive.ApiaryID]
	if !ok {
		return domain.Hive{}, domain.NotFound("蜂场", hive.ApiaryID)
	}
	if _, ok := m.hives[hive.ID]; ok {
		return domain.Hive{}, domain.Conflict("蜂箱 %q 已存在", hive.ID)
	}
	// 归属取本把写锁提交时刻蜂场的当前值守人，与交接在同一把锁上串行，
	// 因此蜂箱的归属在交接两侧二选一，不会出现一名归属两名值守人。
	hive.Keeper = apiary.Keeper
	m.hives[hive.ID] = hive
	return hive, nil
}

func (m *Memory) GetHive(id string) (domain.Hive, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hive, ok := m.hives[id]
	return hive, ok
}

// AdvanceHive 在同一把写锁内完成状态读取、迁移校验与写入，避免并发迁移产生交错结果。
func (m *Memory) AdvanceHive(hiveID string, target domain.HiveState, _ time.Time) (domain.Transition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives[hiveID]
	if !ok {
		return domain.Transition{}, domain.NotFound("蜂箱", hiveID)
	}
	if hive.State == target {
		return domain.Transition{}, domain.InvalidState("蜂箱 %q 已经处于状态 %q", hiveID, target)
	}
	if !domain.CanTransitionHive(hive.State, target) {
		return domain.Transition{}, domain.InvalidState(
			"蜂箱 %q 不允许从 %q 迁移到 %q", hiveID, hive.State, target)
	}
	from := hive.State
	hive.State = target
	m.hives[hiveID] = hive
	return domain.Transition{HiveID: hiveID, From: from, To: target}, nil
}

// AddInspection 分配巡检编号、写入记录，并推进蜂箱的最近巡检时间。
// 蜂箱存在与可接受巡检的判定与写入处于同一把锁，避免退役与巡检之间的竞态。
func (m *Memory) AddInspection(hiveID string, inspection domain.Inspection) (domain.Inspection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives[hiveID]
	if !ok {
		return domain.Inspection{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsInspection() {
		return domain.Inspection{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受巡检", hiveID, hive.State)
	}
	apiary, ok := m.apiaries[hive.ApiaryID]
	if !ok {
		return domain.Inspection{}, domain.Storage("蜂箱 %q 所属蜂场 %q 记录缺失", hiveID, hive.ApiaryID)
	}
	m.inspectionSeq++
	inspection.ID = fmt.Sprintf("inspection-%06d", m.inspectionSeq)
	inspection.HiveID = hiveID
	// 归属取本把写锁提交时刻的当前值守人，与交接串行；
	// 交接之前已固化的巡检记录不会被改写，新巡检按新值守人归属。
	inspection.Keeper = apiary.Keeper
	m.inspections[hiveID] = append(m.inspections[hiveID], inspection)

	observedAt := inspection.ObservedAt
	hive.LastInspectionAt = &observedAt
	m.hives[hiveID] = hive
	return inspection, nil
}

// AddQueen 关闭原有产卵蜂王并写入新蜂王，两件事在同一把锁内完成，
// 保证任意时刻同一蜂箱至多只有一只 laying 蜂王。
func (m *Memory) AddQueen(
	hiveID string,
	queen domain.Queen,
	closeQueenID string,
	closeState domain.QueenState,
	now time.Time,
) (domain.Queen, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives[hiveID]
	if !ok {
		return domain.Queen{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsQueenChange() {
		return domain.Queen{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受换王", hiveID, hive.State)
	}
	if closeQueenID != "" {
		if err := m.closeQueenLocked(hiveID, closeQueenID, closeState, now); err != nil {
			return domain.Queen{}, err
		}
	}
	m.queenSeq++
	queen.ID = fmt.Sprintf("queen-%06d", m.queenSeq)
	queen.HiveID = hiveID
	queen.State = domain.QueenLaying
	m.queens[hiveID] = append(m.queens[hiveID], queen)

	hive.ActiveQueenID = queen.ID
	m.hives[hiveID] = hive
	return queen, nil
}

func (m *Memory) closeQueenLocked(hiveID, queenID string, state domain.QueenState, now time.Time) error {
	items := m.queens[hiveID]
	for index, item := range items {
		if item.ID != queenID {
			continue
		}
		if !item.IsLaying() {
			return domain.Conflict("蜂王 %q 已经关闭", queenID)
		}
		closedAt := now.UTC()
		item.State = state
		item.ClosedAt = &closedAt
		items[index] = item
		m.queens[hiveID] = items
		return nil
	}
	return domain.Storage("蜂箱 %q 上的蜂王 %q 记录缺失", hiveID, queenID)
}

func (m *Memory) ListInspections(hiveID string) ([]domain.Inspection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.hives[hiveID]; !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	items := m.inspections[hiveID]
	out := make([]domain.Inspection, len(items))
	copy(out, items)
	return out, nil
}

func (m *Memory) ListQueens(hiveID string) ([]domain.Queen, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.hives[hiveID]; !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	items := m.queens[hiveID]
	out := make([]domain.Queen, len(items))
	copy(out, items)
	return out, nil
}

// Snapshot 返回深拷贝的一致视图，供汇总类读取使用。
func (m *Memory) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := Snapshot{
		Apiaries:          make(map[string]domain.Apiary, len(m.apiaries)),
		Hives:             make(map[string]domain.Hive, len(m.hives)),
		InspectionsByHive: make(map[string][]domain.Inspection, len(m.inspections)),
		QueensByHive:      make(map[string][]domain.Queen, len(m.queens)),
		HandoversByApiary: make(map[string][]domain.KeeperHandover, len(m.handovers)),
	}
	for id, apiary := range m.apiaries {
		snapshot.Apiaries[id] = apiary
	}
	for id, hive := range m.hives {
		snapshot.Hives[id] = cloneHive(hive)
	}
	for id, items := range m.inspections {
		copied := make([]domain.Inspection, len(items))
		copy(copied, items)
		for index := range copied {
			copied[index].Concerns = append([]string{}, items[index].Concerns...)
		}
		snapshot.InspectionsByHive[id] = copied
	}
	for id, items := range m.queens {
		copied := make([]domain.Queen, len(items))
		copy(copied, items)
		for index := range copied {
			if items[index].ClosedAt != nil {
				closedAt := *items[index].ClosedAt
				copied[index].ClosedAt = &closedAt
			}
		}
		snapshot.QueensByHive[id] = copied
	}
	for id, items := range m.handovers {
		copied := make([]domain.KeeperHandover, len(items))
		copy(copied, items)
		snapshot.HandoversByApiary[id] = copied
	}
	return snapshot
}

func cloneHive(hive domain.Hive) domain.Hive {
	if hive.LastInspectionAt != nil {
		observedAt := *hive.LastInspectionAt
		hive.LastInspectionAt = &observedAt
	}
	return hive
}
