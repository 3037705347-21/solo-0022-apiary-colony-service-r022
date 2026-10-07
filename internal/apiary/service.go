package apiary

import (
	"time"

	"example.com/apiary-colony-service/internal/domain"
	"example.com/apiary-colony-service/internal/store"
)

// Service 负责蜂场与蜂箱的录入，以及蜂箱状态迁移。
type Service struct {
	store *store.Memory
	clock func() time.Time
}

func New(memory *store.Memory, clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: memory, clock: clock}
}

type RegisterApiaryInput struct {
	ID       string
	Name     string
	District string
	Keeper   string
}

type InstallHiveInput struct {
	ID    string
	Model string
}

type HandoverKeeperInput struct {
	FromKeeper   string
	ToKeeper     string
	HandedOverAt time.Time
}

func (s *Service) RegisterApiary(input RegisterApiaryInput) (domain.Apiary, error) {
	apiary, err := domain.NewApiary(input.ID, input.Name, input.District, input.Keeper, s.clock())
	if err != nil {
		return domain.Apiary{}, err
	}
	if err := s.store.CreateApiary(apiary); err != nil {
		return domain.Apiary{}, err
	}
	return apiary, nil
}

func (s *Service) Apiary(id string) (domain.Apiary, error) {
	apiary, ok := s.store.GetApiary(id)
	if !ok {
		return domain.Apiary{}, domain.NotFound("蜂场", id)
	}
	return apiary, nil
}

func (s *Service) InstallHive(apiaryID string, input InstallHiveInput) (domain.Hive, error) {
	if _, err := s.Apiary(apiaryID); err != nil {
		return domain.Hive{}, err
	}
	model, err := ParseHiveModel(input.Model)
	if err != nil {
		return domain.Hive{}, err
	}
	hive, err := domain.NewHive(input.ID, apiaryID, model, s.clock())
	if err != nil {
		return domain.Hive{}, err
	}
	stored, err := s.store.CreateHive(hive)
	if err != nil {
		return domain.Hive{}, err
	}
	return stored, nil
}

func (s *Service) Hive(id string) (domain.Hive, error) {
	hive, ok := s.store.GetHive(id)
	if !ok {
		return domain.Hive{}, domain.NotFound("蜂箱", id)
	}
	return hive, nil
}

func (s *Service) AdvanceHiveState(hiveID, target string) (domain.Transition, error) {
	to, err := ParseHiveState(target)
	if err != nil {
		return domain.Transition{}, err
	}
	return s.store.AdvanceHive(hiveID, to, s.clock())
}

// HandoverKeeper 由协调人办理值守人交接：校验字段后交给仓储在同一把写锁内
// 核对当前值守人、写入交接记录并切换蜂场当前值守人。
func (s *Service) HandoverKeeper(apiaryID string, input HandoverKeeperInput) (domain.KeeperHandover, error) {
	handedOverAt := input.HandedOverAt
	if handedOverAt.IsZero() {
		handedOverAt = s.clock()
	}
	handover, err := domain.NewKeeperHandover(apiaryID, input.FromKeeper, input.ToKeeper, handedOverAt)
	if err != nil {
		return domain.KeeperHandover{}, err
	}
	return s.store.HandoverKeeper(handover)
}

// Handovers 读回蜂场的交接链，保留每一次交接的时间与双方信息。
func (s *Service) Handovers(apiaryID string) ([]domain.KeeperHandover, error) {
	return s.store.ListHandovers(apiaryID)
}
