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
	ToKeeper string
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

// HandoverKeeper 办理值守人交接：记录交出方、接任方与交接时间，
// 并把蜂场当前值守人切换为接任者；交接前已形成的记录保持原归属。
func (s *Service) HandoverKeeper(apiaryID string, input HandoverKeeperInput) (domain.KeeperHandover, error) {
	return s.store.HandoverKeeper(apiaryID, input.ToKeeper, s.clock())
}

func (s *Service) KeeperHandovers(apiaryID string) ([]domain.KeeperHandover, error) {
	return s.store.ListKeeperHandovers(apiaryID)
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
	return s.store.CreateHive(hive)
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
