package consorcio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mclara9593/caronasDistribuidas/internal/graph"
	"github.com/mclara9593/caronasDistribuidas/internal/protocol"
)

type ItemItinerario struct {
	SectionID string
	Assentos  int
}

func gerarTransacaoID(idServidor string) string {
	return fmt.Sprintf("res-%s-%d", idServidor, time.Now().UnixNano())
}

func CoordenarReserva(
	itinerario []ItemItinerario,
	passageiroEmail string,
	meuIDServidor string,
	relogio *RelogioLogico,
	grafo *graph.Grafo,
	peers *PeersRegistry,
) (string, error) {
	if len(itinerario) == 0 {
		return "", fmt.Errorf("itinerário sem trechos")
	}
	if passageiroEmail == "" || meuIDServidor == "" {
		return "", fmt.Errorf("email do passageiro e ID do servidor são obrigatórios")
	}
	if relogio == nil || grafo == nil || peers == nil {
		return "", fmt.Errorf("coordenador não inicializado")
	}

	porServidor := make(map[string][]protocol.TransactionSection)
	var ordem []string
	vistos := make(map[string]bool)

	for _, item := range itinerario {
		idLocal, servidor, err := protocol.DecomporSectionID(item.SectionID)
		if err != nil {
			return "", err
		}
		if idLocal == "" || servidor == "" {
			return "", fmt.Errorf("ID de trecho incompleto: %q", item.SectionID)
		}
		if item.Assentos != 1 {
			return "", fmt.Errorf("a reserva deve ocupar um assento por trecho")
		}
		if vistos[item.SectionID] {
			return "", fmt.Errorf("trecho repetido no itinerário: %s", item.SectionID)
		}
		vistos[item.SectionID] = true

		if _, existe := porServidor[servidor]; !existe {
			ordem = append(ordem, servidor)
		}
		porServidor[servidor] = append(
			porServidor[servidor],
			protocol.TransactionSection{
				SectionID: item.SectionID,
				Seats:     1,
			},
		)
	}

	txID := gerarTransacaoID(meuIDServidor)
	timestamp := relogio.Tick()
	preparados := make([]string, 0, len(ordem))

	for _, servidor := range ordem {
		req := protocol.TransactionRequest{
			TransactionID:    txID,
			PassengerEmail:   passageiroEmail,
			OriginServer:     meuIDServidor,
			LamportTimestamp: timestamp,
			Sections:         porServidor[servidor],
		}

		resp, err := enviarTransacao(servidor, "prepare", req, meuIDServidor, relogio, grafo, peers)
		if err != nil || !resp.Accepted {
			paraAbortar := append(append([]string(nil), preparados...), servidor)
			abortar(paraAbortar, porServidor, txID, passageiroEmail,
				meuIDServidor, timestamp, relogio, grafo, peers)

			if err != nil {
				return "", fmt.Errorf("prepare no servidor %s: %w", servidor, err)
			}
			return "", fmt.Errorf("prepare recusado por %s: %s", servidor, resp.Reason)
		}
		preparados = append(preparados, servidor)
	}

	// Depois que todos aceitaram prepare, não se deve abortar parcialmente.
	for _, servidor := range preparados {
		req := protocol.TransactionRequest{
			TransactionID:    txID,
			PassengerEmail:   passageiroEmail,
			OriginServer:     meuIDServidor,
			LamportTimestamp: timestamp,
			Sections:         porServidor[servidor],
		}

		resp, err := enviarTransacao(servidor, "commit", req, meuIDServidor, relogio, grafo, peers)
		if err != nil {
			return txID, fmt.Errorf("reserva %s pendente: commit em %s: %w", txID, servidor, err)
		}
		if !resp.Accepted {
			return txID, fmt.Errorf("commit recusado por %s: %s", servidor, resp.Reason)
		}
	}

	return txID, nil
}

func abortar(
	servidores []string,
	porServidor map[string][]protocol.TransactionSection,
	txID, email, meuID string,
	timestamp uint64,
	relogio *RelogioLogico,
	grafo *graph.Grafo,
	peers *PeersRegistry,
) {
	for _, servidor := range servidores {
		req := protocol.TransactionRequest{
			TransactionID:    txID,
			PassengerEmail:   email,
			OriginServer:     meuID,
			LamportTimestamp: timestamp,
			Sections:         porServidor[servidor],
		}
		_, _ = enviarTransacao(servidor, "abort", req, meuID, relogio, grafo, peers)
	}
}

func enviarTransacao(
	servidor, acao string,
	req protocol.TransactionRequest,
	meuID string,
	relogio *RelogioLogico,
	grafo *graph.Grafo,
	peers *PeersRegistry,
) (protocol.TransactionResponse, error) {
	if servidor == meuID {
		switch acao {
		case "prepare":
			return TratarPrepare(grafo, relogio, meuID, req), nil
		case "commit":
			return TratarCommit(grafo, relogio, meuID, req), nil
		case "abort":
			return TratarAbort(grafo, relogio, meuID, req), nil
		default:
			return protocol.TransactionResponse{}, fmt.Errorf("ação desconhecida: %s", acao)
		}
	}

	urlBase, existe := peers.URLDoServidor(servidor)
	if !existe {
		return protocol.TransactionResponse{}, fmt.Errorf("servidor %s desconhecido", servidor)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return protocol.TransactionResponse{}, err
	}

	endpoint := strings.TrimRight(urlBase, "/") + "/v1/transactions/" + acao
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return protocol.TransactionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return protocol.TransactionResponse{}, err
	}
	defer httpResp.Body.Close()

	var resposta protocol.TransactionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resposta); err != nil {
		return resposta, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return resposta, fmt.Errorf("servidor %s respondeu HTTP %s", servidor, httpResp.Status)
	}
	if resposta.TransactionID != req.TransactionID {
		return resposta, fmt.Errorf("resposta com ID de transação inesperado de %s", servidor)
	}
	return resposta, nil
}

func CoordenarCancelamento(
	reservationID string,
	passageiroEmail string,
	trechos []ItemItinerario,
	meuIDServidor string,
	relogio *RelogioLogico,
	grafo *graph.Grafo,
	peers *PeersRegistry,
) error {
	if reservationID == "" || passageiroEmail == "" || meuIDServidor == "" {
		return fmt.Errorf("ID da reserva, email do passageiro e servidor são obrigatórios")
	}
	if len(trechos) == 0 {
		return fmt.Errorf("reserva sem trechos")
	}
	if relogio == nil || grafo == nil || peers == nil {
		return fmt.Errorf("coordenador não inicializado")
	}

	porServidor := make(map[string][]protocol.TransactionSection)
	var ordem []string
	vistos := make(map[string]bool)

	for _, item := range trechos {
		idLocal, servidor, err := protocol.DecomporSectionID(item.SectionID)
		if err != nil {
			return err
		}
		if idLocal == "" || servidor == "" {
			return fmt.Errorf("ID de trecho incompleto: %q", item.SectionID)
		}
		if vistos[item.SectionID] {
			return fmt.Errorf("trecho duplicado: %s", item.SectionID)
		}
		vistos[item.SectionID] = true

		if _, existe := porServidor[servidor]; !existe {
			ordem = append(ordem, servidor)
		}
		porServidor[servidor] = append(
			porServidor[servidor],
			protocol.TransactionSection{SectionID: item.SectionID, Seats: 1},
		)
	}

	timestamp := relogio.Tick()
	preparados := make([]string, 0, len(ordem))

	enviar := func(servidor, acao string) (protocol.TransactionResponse, error) {
		req := protocol.TransactionRequest{
			TransactionID:    reservationID,
			PassengerEmail:   passageiroEmail,
			OriginServer:     meuIDServidor,
			LamportTimestamp: timestamp,
			Sections:         porServidor[servidor],
		}
		return enviarTransacaoCancelamento(
			servidor, acao, req, meuIDServidor, relogio, grafo, peers,
		)
	}

	for _, servidor := range ordem {
		resp, err := enviar(servidor, "prepare-cancel")
		if err != nil || !resp.Accepted {
			// Aborta todos os prepares aceitos. Se o prepare que falhou
			// chegou ao peer, o abort idempotente também limpa a pendência.
			paraAbortar := append(append([]string(nil), preparados...), servidor)
			for _, servidorAbort := range paraAbortar {
				_, _ = enviar(servidorAbort, "abort-cancel")
			}
			if err != nil {
				return fmt.Errorf("prepare de cancelamento em %s: %w", servidor, err)
			}
			return fmt.Errorf("prepare de cancelamento recusado por %s: %s", servidor, resp.Reason)
		}
		preparados = append(preparados, servidor)
	}

	// Após todos os prepares, tenta confirmar em todos os participantes.
	// Uma falha aqui é reportada para evitar remover a carona parcialmente.
	for _, servidor := range preparados {
		resp, err := enviar(servidor, "commit-cancel")
		if err != nil {
			return fmt.Errorf("cancelamento %s pode estar pendente em %s: %w", reservationID, servidor, err)
		}
		if !resp.Accepted {
			return fmt.Errorf("commit de cancelamento recusado por %s: %s", servidor, resp.Reason)
		}
	}

	return nil
}

func enviarTransacaoCancelamento(
	servidor, acao string,
	req protocol.TransactionRequest,
	meuID string,
	relogio *RelogioLogico,
	grafo *graph.Grafo,
	peers *PeersRegistry,
) (protocol.TransactionResponse, error) {
	if servidor == meuID {
		switch acao {
		case "prepare-cancel":
			return TratarPrepareCancelamento(grafo, relogio, meuID, req), nil
		case "commit-cancel":
			return TratarCommitCancelamento(grafo, relogio, meuID, req), nil
		case "abort-cancel":
			return TratarAbortCancelamento(grafo, relogio, meuID, req), nil
		default:
			return protocol.TransactionResponse{}, fmt.Errorf("ação de cancelamento desconhecida: %s", acao)
		}
	}

	urlBase, existe := peers.URLDoServidor(servidor)
	if !existe {
		return protocol.TransactionResponse{}, fmt.Errorf("servidor %s desconhecido", servidor)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return protocol.TransactionResponse{}, err
	}

	endpoint := strings.TrimRight(urlBase, "/") +
		"/v1/transactions/" + acao
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return protocol.TransactionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return protocol.TransactionResponse{}, err
	}
	defer httpResp.Body.Close()

	var resposta protocol.TransactionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resposta); err != nil {
		return resposta, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return resposta, fmt.Errorf("servidor %s respondeu HTTP %s", servidor, httpResp.Status)
	}
	if resposta.TransactionID != req.TransactionID {
		return resposta, fmt.Errorf("resposta com ID de transação inesperado de %s", servidor)
	}

	return resposta, nil
}
