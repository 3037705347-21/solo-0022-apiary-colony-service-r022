package httpapi

import (
	"net/http"

	"example.com/apiary-colony-service/internal/apiary"
	"example.com/apiary-colony-service/internal/domain"
)

type handoverKeeperRequest struct {
	ToKeeper string `json:"toKeeper"`
}

func (s *Server) handleHandoverKeeper(w http.ResponseWriter, r *http.Request) {
	var request handoverKeeperRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	handover, err := s.deps.Apiary.HandoverKeeper(r.PathValue("apiaryId"), apiary.HandoverKeeperInput{
		ToKeeper: request.ToKeeper,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, handover)
}

func (s *Server) handleListKeeperHandovers(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Apiary.KeeperHandovers(r.PathValue("apiaryId"))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []domain.KeeperHandover{}
	}
	writeJSON(w, http.StatusOK, items)
}
