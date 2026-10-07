package domain

import "time"

// KeeperHandover 是一次值守人交接的固化记录，写入后不可修改。
// FromKeeper/ToKeeper 记录交接双方，HandedOverAt 记录交接时刻；
// 交接只影响此后新建的蜂箱与巡检，此前的记录保留原值守人归属。
type KeeperHandover struct {
	ID           string    `json:"id"`
	ApiaryID     string    `json:"apiaryId"`
	FromKeeper   string    `json:"fromKeeper"`
	ToKeeper     string    `json:"toKeeper"`
	HandedOverAt time.Time `json:"handedOverAt"`
}

// NewKeeperHandover 在进入仓储前完成字段校验；
// 当前值守人与输入是否一致、编号分配与正式写入由仓储在写锁内完成。
func NewKeeperHandover(apiaryID, fromKeeper, toKeeper string, handedOverAt time.Time) (KeeperHandover, error) {
	if err := ValidateIdentifier("apiaryId", apiaryID, 3, 40); err != nil {
		return KeeperHandover{}, err
	}
	if err := ValidateText("fromKeeper", fromKeeper, 1, 40); err != nil {
		return KeeperHandover{}, err
	}
	if err := ValidateText("toKeeper", toKeeper, 1, 40); err != nil {
		return KeeperHandover{}, err
	}
	if handedOverAt.IsZero() {
		return KeeperHandover{}, Invalid("handedOverAt", "handedOverAt 不能为空")
	}
	if TrimText(fromKeeper) == TrimText(toKeeper) {
		return KeeperHandover{}, Invalid("toKeeper", "toKeeper 不能与 fromKeeper 相同")
	}
	return KeeperHandover{
		ApiaryID:     TrimText(apiaryID),
		FromKeeper:   TrimText(fromKeeper),
		ToKeeper:     TrimText(toKeeper),
		HandedOverAt: handedOverAt.UTC(),
	}, nil
}
