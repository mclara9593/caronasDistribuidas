package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Request struct {
	Action    string          `json:"action"`     // Ex: "auth_conductor", "publish_ride"
	RequestID string          `json:"request_id"` // ID único gerado no cliente
	Payload   json.RawMessage `json:"payload"`    // Bytes brutos do JSON interno
}

type Response struct {
	RequestID string          `json:"request_id"` // Retorna o mesmo ID para o cliente se achar
	Status    string          `json:"status"`     // "SUCCESS" ou "ERROR"
	Message   string          `json:"message"`    // Descrição ou erro
	Payload   json.RawMessage `json:"payload"`    // Opcional: dados de retorno (ex: lista de caronas)
}

// ============================
// CADASTRO E LOGIN
// ============================

// CadastroPassageiro é o payload enviado para criar uma conta de passageiro
type CadastroPassageiro struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// CadastroMotorista é o payload enviado para criar uma conta de motorista.
// É igual ao de passageiro, acrescido apenas da CNH.
type CadastroMotorista struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
	CNH   string `json:"cnh"`
}

// LoginRequest é usado tanto por passageiros quanto por motoristas para autenticar
type LoginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// LoginResult é devolvido pelo servidor dentro do Payload da Response quando o login dá certo,
// para que o cliente saiba o nome de quem acabou de logar
type LoginResult struct {
	Nome string `json:"nome"`
}

// ============================
// CARONAS / RESERVAS
// ============================

type PushRide struct {
	Route         []string  `json:"route"`
	DepartureTime string    `json:"departure_time"`
	Capacity      int       `json:"available_seats"`
	Precos        []float64 `json:"precos"`       // um preço por trecho: Precos[i] é o preço de Route[i] -> Route[i+1]
	DriverEmail   string    `json:"driver_email"` // email do motorista logado que está publicando
}

// GetMyRidesRequest é usado pelo motorista pra consultar as caronas que ele mesmo publicou
type GetMyRidesRequest struct {
	Email string `json:"email"`
}

// SectionInfo / RideInfo espelham graph.Section / graph.Ride (mesmas tags JSON), usados nas
// respostas de consulta, para o cliente não precisar importar o pacote internal/graph.
type SectionInfo struct {
	IdSection string  `json:"id"`
	Origem    string  `json:"origem"`
	Destino   string  `json:"destino"`
	Assentos  int     `json:"assentos"`
	Preco     float64 `json:"preco"`
}

type RideInfo struct {
	IdRide     string        `json:"id"`
	Sections   []SectionInfo `json:"Sections"`
	IdDriver   string        `json:"id_driver"`
	TotalPrice float64       `json:"total_price"`
}

// PublishRideResult é devolvido ao motorista assim que a carona é cadastrada no Grafo,
// para que ele saiba qual ID usar depois pra cancelar ou consultar essa carona.
type PublishRideResult struct {
	RideID string `json:"ride_id"`
}

// BookRideResult é devolvido ao passageiro assim que a reserva é confirmada,
// para que ele saiba qual ID usar depois pra cancelar essa reserva.
type BookRideResult struct {
	RideID string `json:"ride_id"`
}

// Cancelar e Consultar carona por motorista
type CancelSearchRide struct {
	Route  []string `json:"route"`
	RideID string   `json:"ride_id"`
}

// Buscar itinerários por usuário
type SearchRoute struct {
	Destination string `json:"destination"`
	Source      string `json:"source"`
	ArrivalTime string `json:"arrival_time"`
}

// Reserva, Consulta de reserva, e cancelamento por usuário
type GetID struct {
	RideID string `json:"ride_id"`
}

// ============================
// Outros servidores
// ============================

// separadorServidor une o id do trecho e o id do servidor dono num
// único identificador trafegado pelo protocolo entre servidores,
// evitando a necessidade de dois campos separados nas mensagens.
const separadorServidor = "@"

// ComporSectionID junta o id local de um trecho com o id do servidor
// dono, formando o identificador que efetivamente trafega nas
// mensagens do protocolo (buscas, prepare, commit, abort).
func ComporSectionID(idTrecho, idServidor string) string {
	return fmt.Sprintf("%s%s%s", idTrecho, separadorServidor, idServidor)
}

// DecomporSectionID separa um identificador recebido pela rede de
// volta em id do trecho e id do servidor dono. Retorna erro se o
// identificador não estiver no formato esperado.
func DecomporSectionID(idCompleto string) (idTrecho string, idServidor string, err error) {
	partes := strings.SplitN(idCompleto, separadorServidor, 2)
	if len(partes) != 2 {
		return "", "", fmt.Errorf("id de trecho mal formado, esperado 'trecho%sservidor': %q", separadorServidor, idCompleto)
	}
	return partes[0], partes[1], nil
}

type GiveRoute struct {
	Destination string   `json:"destination"`
	Source      string   `json:"source"`
	Route       []string `json:"route_by_sender"`
}
