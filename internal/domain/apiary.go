package domain

import "time"

// Apiary 表示一座蜂场。
type Apiary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	District  string    `json:"district"`
	Keeper    string    `json:"keeper"`
	CreatedAt time.Time `json:"createdAt"`
}

// KeeperHandover 记录一次值守人交接：交出方、接任方与交接时间。
// 交接完成后蜂场的当前值守人切换为接任者，历史记录保持原归属。
type KeeperHandover struct {
	ID           string    `json:"id"`
	ApiaryID     string    `json:"apiaryId"`
	FromKeeper   string    `json:"fromKeeper"`
	ToKeeper     string    `json:"toKeeper"`
	HandedOverAt time.Time `json:"handedOverAt"`
}

func NewApiary(id, name, district, keeper string, createdAt time.Time) (Apiary, error) {
	if err := ValidateIdentifier("id", id, 3, 40); err != nil {
		return Apiary{}, err
	}
	if err := ValidateText("name", name, 1, 60); err != nil {
		return Apiary{}, err
	}
	if err := ValidateText("district", district, 1, 60); err != nil {
		return Apiary{}, err
	}
	if err := ValidateText("keeper", keeper, 1, 40); err != nil {
		return Apiary{}, err
	}
	return Apiary{
		ID:        TrimText(id),
		Name:      TrimText(name),
		District:  TrimText(district),
		Keeper:    TrimText(keeper),
		CreatedAt: createdAt.UTC(),
	}, nil
}

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
	return KeeperHandover{
		ApiaryID:     TrimText(apiaryID),
		FromKeeper:   TrimText(fromKeeper),
		ToKeeper:     TrimText(toKeeper),
		HandedOverAt: handedOverAt.UTC(),
	}, nil
}
