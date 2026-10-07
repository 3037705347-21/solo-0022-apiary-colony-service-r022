package httpapi

import (
	"net/http"

	"example.com/apiary-colony-service/internal/apiary"
	"example.com/apiary-colony-service/internal/domain"
)

type registerApiaryRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	District string `json:"district"`
	Keeper   string `json:"keeper"`
}

type handoverKeeperRequest struct {
	FromKeeper   string `json:"fromKeeper"`
	ToKeeper     string `json:"toKeeper"`
	HandedOverAt string `json:"handedOverAt"`
}

func (s *Server) handleRegisterApiary(w http.ResponseWriter, r *http.Request) {
	var request registerApiaryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	created, err := s.deps.Apiary.RegisterApiary(apiary.RegisterApiaryInput{
		ID:       request.ID,
		Name:     request.Name,
		District: request.District,
		Keeper:   request.Keeper,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetApiary(w http.ResponseWriter, r *http.Request) {
	found, err := s.deps.Apiary.Apiary(r.PathValue("apiaryId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, found)
}

func (s *Server) handleHandoverKeeper(w http.ResponseWriter, r *http.Request) {
	var request handoverKeeperRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	handedOverAt, err := parseOptionalTime("handedOverAt", request.HandedOverAt)
	if err != nil {
		writeError(w, err)
		return
	}
	input := apiary.HandoverKeeperInput{
		FromKeeper: request.FromKeeper,
		ToKeeper:   request.ToKeeper,
	}
	if handedOverAt != nil {
		input.HandedOverAt = *handedOverAt
	}
	handover, err := s.deps.Apiary.HandoverKeeper(r.PathValue("apiaryId"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, handover)
}

func (s *Server) handleListHandovers(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Apiary.Handovers(r.PathValue("apiaryId"))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []domain.KeeperHandover{}
	}
	writeJSON(w, http.StatusOK, items)
}
