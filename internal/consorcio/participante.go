package consorcio

import (
	"fmt"

	"github.com/mclara9593/caronasDistribuidas/internal/graph"
	"github.com/mclara9593/caronasDistribuidas/internal/protocol"
)

func TratarPrepare(
	grafo *graph.Grafo,
	relogio *RelogioLogico,
	meuIDServidor string,
	req protocol.TransactionRequest,
) protocol.TransactionResponse {
	if err := validarParticipante(grafo, relogio, meuIDServidor, req); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	relogio.Observar(req.LamportTimestamp)

	ids, err := idsLocais(req.Sections, meuIDServidor)
	if err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	return respostaParticipante(req.TransactionID,
		grafo.PrepareReservation(req.TransactionID, req.PassengerEmail, ids))
}

func TratarCommit(
	grafo *graph.Grafo,
	relogio *RelogioLogico,
	meuIDServidor string,
	req protocol.TransactionRequest,
) protocol.TransactionResponse {
	if err := validarParticipante(grafo, relogio, meuIDServidor, req); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	relogio.Observar(req.LamportTimestamp)

	if _, err := idsLocais(req.Sections, meuIDServidor); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	return respostaParticipante(req.TransactionID, grafo.CommitReservation(req.TransactionID))
}

func TratarAbort(
	grafo *graph.Grafo,
	relogio *RelogioLogico,
	meuIDServidor string,
	req protocol.TransactionRequest,
) protocol.TransactionResponse {
	if err := validarParticipante(grafo, relogio, meuIDServidor, req); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	relogio.Observar(req.LamportTimestamp)

	if _, err := idsLocais(req.Sections, meuIDServidor); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	return respostaParticipante(req.TransactionID, grafo.AbortReservation(req.TransactionID))
}

func validarParticipante(
	grafo *graph.Grafo,
	relogio *RelogioLogico,
	meuID string,
	req protocol.TransactionRequest,
) error {
	if grafo == nil || relogio == nil || meuID == "" {
		return fmt.Errorf("participante não inicializado")
	}
	if req.TransactionID == "" || req.PassengerEmail == "" {
		return fmt.Errorf("ID da transação e email são obrigatórios")
	}
	return nil
}

func idsLocais(
	sections []protocol.TransactionSection,
	meuID string,
) ([]string, error) {
	if len(sections) == 0 {
		return nil, fmt.Errorf("requisição sem trechos")
	}

	ids := make([]string, 0, len(sections))
	vistos := make(map[string]bool)

	for _, section := range sections {
		if section.Seats != 1 {
			return nil, fmt.Errorf("cada trecho deve reservar exatamente um assento")
		}
		idLocal, servidor, err := protocol.DecomporSectionID(section.SectionID)
		if err != nil {
			return nil, err
		}
		if servidor != meuID {
			return nil, fmt.Errorf("este servidor não é dono do trecho %s", section.SectionID)
		}
		if vistos[idLocal] {
			return nil, fmt.Errorf("trecho duplicado: %s", idLocal)
		}
		vistos[idLocal] = true
		ids = append(ids, idLocal)
	}
	return ids, nil
}

func respostaParticipante(id string, err error) protocol.TransactionResponse {
	if err != nil {
		return protocol.TransactionResponse{
			TransactionID: id,
			Accepted:      false,
			Reason:        err.Error(),
		}
	}
	return protocol.TransactionResponse{TransactionID: id, Accepted: true}
}

func TratarPrepareCancelamento(
	grafo *graph.Grafo,
	relogio *RelogioLogico,
	meuIDServidor string,
	req protocol.TransactionRequest,
) protocol.TransactionResponse {
	if err := validarParticipante(grafo, relogio, meuIDServidor, req); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	relogio.Observar(req.LamportTimestamp)

	ids, err := idsLocais(req.Sections, meuIDServidor)
	if err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	return respostaParticipante(req.TransactionID,
		grafo.PrepareCancellation(req.TransactionID, req.PassengerEmail, ids))
}

func TratarCommitCancelamento(
	grafo *graph.Grafo,
	relogio *RelogioLogico,
	meuIDServidor string,
	req protocol.TransactionRequest,
) protocol.TransactionResponse {
	if err := validarParticipante(grafo, relogio, meuIDServidor, req); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	relogio.Observar(req.LamportTimestamp)

	if _, err := idsLocais(req.Sections, meuIDServidor); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	return respostaParticipante(req.TransactionID, grafo.CommitCancellation(req.TransactionID))
}

func TratarAbortCancelamento(
	grafo *graph.Grafo,
	relogio *RelogioLogico,
	meuIDServidor string,
	req protocol.TransactionRequest,
) protocol.TransactionResponse {
	if err := validarParticipante(grafo, relogio, meuIDServidor, req); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	relogio.Observar(req.LamportTimestamp)

	if _, err := idsLocais(req.Sections, meuIDServidor); err != nil {
		return respostaParticipante(req.TransactionID, err)
	}
	return respostaParticipante(req.TransactionID, grafo.AbortCancellation(req.TransactionID, req.PassengerEmail))
}
