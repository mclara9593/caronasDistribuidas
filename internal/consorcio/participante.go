package consorcio

import (
	"github.com/mclara9593/caronas/internal/domain"
	"github.com/mclara9593/caronas/internal/graph"
	"github.com/mclara9593/caronas/internal/protocol"
)

func TratarPrepare(grafo *graph.Grafo, repo *domain.RepositorioReservas, relogio *RelogioLogico, meuIDServidor string, req RequisicaoReserva) RespostaPrepare {
	relogio.Observar(req.TimestampLamport)

	idTrecho, idServidorAlvo, err := protocol.DecomporSectionID(req.SectionID)
	if err != nil {
		return RespostaPrepare{TransacaoID: req.TransacaoID, Aceito: false, Motivo: err.Error()}
	}

	if idServidorAlvo != meuIDServidor {
		return RespostaPrepare{TransacaoID: req.TransacaoID, Aceito: false, Motivo: "este servidor não é dono do trecho informado"}
	}

	trecho := grafo.BuscarSection(idTrecho)
	if trecho == nil {
		return RespostaPrepare{TransacaoID: req.TransacaoID, Aceito: false, Motivo: "trecho não encontrado"}
	}

	reserva := &domain.ReservaPendente{
		TransacaoID:      req.TransacaoID,
		SectionID:        idTrecho,
		Assentos:         1,
		TimestampLamport: req.TimestampLamport,
		ServidorOrigem:   req.ServidorOrigem,
	}

	if err := repo.Preparar(trecho, reserva); err != nil {
		return RespostaPrepare{TransacaoID: req.TransacaoID, Aceito: false, Motivo: err.Error()}
	}

	return RespostaPrepare{TransacaoID: req.TransacaoID, Aceito: true}
}
