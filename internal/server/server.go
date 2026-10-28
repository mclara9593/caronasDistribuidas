package server

import (
	"encoding/json"
	"net/http"

	"github.com/mclara9593/caronas/internal/protocol"
)

// SectionResposta é o formato que efetivamente trafega entre
// servidores, com o id já composto pelo trecho + servidor dono,
// evitando dois campos separados na mensagem.
type SectionResposta struct {
	SectionID string  `json:"section_id"`
	Origem    string  `json:"origem"`
	Destino   string  `json:"destino"`
	Assentos  int     `json:"assentos"`
	Preco     float64 `json:"preco"`
}

func (s *Server) handleConsultarTrechos(w http.ResponseWriter, r *http.Request) {
	origem := r.URL.Query().Get("origem")
	destino := r.URL.Query().Get("destino")

	if origem == "" || destino == "" {
		http.Error(w, "parâmetros 'origem' e 'destino' são obrigatórios", http.StatusBadRequest)
		return
	}

	estado := s.grafo.Exportar()

	var resposta []SectionResposta
	for _, trechos := range estado.Nodes {
		for _, trecho := range trechos {
			if trecho.Origem == origem && trecho.Destino == destino && trecho.Assentos > 0 {
				resposta = append(resposta, SectionResposta{
					SectionID: protocol.ComporSectionID(trecho.ID, s.idServidor),
					Origem:    trecho.Origem,
					Destino:   trecho.Destino,
					Assentos:  trecho.Assentos,
					Preco:     trecho.Preco,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resposta)
}
