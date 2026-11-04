package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/mclara9593/caronasDistribuidas/internal/consorcio"
	"github.com/mclara9593/caronasDistribuidas/internal/domain"
	"github.com/mclara9593/caronasDistribuidas/internal/graph"
	"github.com/mclara9593/caronasDistribuidas/internal/persistence"
	"github.com/mclara9593/caronasDistribuidas/internal/protocol"
)

type SectionResposta struct {
	SectionID string  `json:"section_id"`
	Origem    string  `json:"origem"`
	Destino   string  `json:"destino"`
	Assentos  int     `json:"assentos"`
	Preco     float64 `json:"preco"`
}

type Config struct {
	IDServidor string
	TCPAddr    string
	HTTPAddr   string
	Peers      map[string]string
}

type Server struct {
	idServidor string
	peers      map[string]string

	grafo           *graph.Grafo
	repoPassageiros *domain.Repositorio
	repoMotoristas  *domain.Repositorio

	tcpAddr       string
	httpAddr      string
	relogio       *consorcio.RelogioLogico
	peersRegistry *consorcio.PeersRegistry
}

func NovoServidor(cfg Config) *Server {
	return &Server{
		idServidor:      cfg.IDServidor,
		peers:           cfg.Peers,
		tcpAddr:         cfg.TCPAddr,
		httpAddr:        cfg.HTTPAddr,
		grafo:           graph.NovoGrafo(),
		repoPassageiros: domain.NovoRepositorio(),
		repoMotoristas:  domain.NovoRepositorio(),
		relogio:         consorcio.NovoRelogio(cfg.IDServidor),
		peersRegistry:   peersRegistry(cfg.Peers),
	}
}

func peersRegistry(peers map[string]string) *consorcio.PeersRegistry {
	registry := consorcio.NovoPeersRegistry(peers)
	return &registry
}

func (s *Server) Iniciar() error {
	if err := persistence.CarregarEstado(s.idServidor, s.grafo); err != nil {
		return fmt.Errorf("carregar backup local: %w", err)
	}

	tcpListener, err := net.Listen("tcp", s.tcpAddr)
	if err != nil {
		return fmt.Errorf("erro ao iniciar TCP em %s: %w", s.tcpAddr, err)
	}
	defer tcpListener.Close()

	mux := http.NewServeMux()
	s.registrarRotasHTTP(mux)

	httpServer := &http.Server{
		Addr:              s.httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Erro no listener REST: %v\n", err)
		}
	}()

	fmt.Printf("Empresa %s: TCP %s, REST %s\n", s.idServidor, s.tcpAddr, s.httpAddr)

	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			fmt.Printf("Erro ao aceitar conexão TCP: %v\n", err)
			continue
		}
		go s.tratarCliente(conn)
	}
}

func (s *Server) salvarEstado() error {
	return persistence.SalvarEstado(s.idServidor, s.grafo)
}

func (s *Server) tratarCliente(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		linha, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Erro de leitura do cliente %s: %v\n", conn.RemoteAddr(), err)
			}
			return
		}

		var req protocol.Request
		if err := json.Unmarshal(linha, &req); err != nil {
			s.enviarErro(conn, "", "Formato JSON inválido")
			continue
		}
		s.rotearRequisicao(conn, &req)
	}
}

func (s *Server) rotearRequisicao(conn net.Conn, req *protocol.Request) {
	switch req.Action {
	case "cadastrar_user":
		s.handleCadastrarUser(conn, req)
	case "cadastrar_motorista":
		s.handleCadastrarMotorista(conn, req)
	case "auth_user":
		s.handleAuthUser(conn, req)
	case "auth_conductor":
		s.handleAuthConductor(conn, req)
	case "publish_ride":
		s.handlePublishRide(conn, req)
	case "cancel_ride":
		s.handleCancelRide(conn, req)
	case "get_my_rides":
		s.handleGetMyRides(conn, req)
	case "search_route":
		s.handleSearchRoute(conn, req)
	case "book_ride":
		s.handleBookRide(conn, req)
	case "get_bookings":
		s.handleGetBookings(conn, req)
	case "cancel_booking":
		s.handleCancelBooking(conn, req)
	default:
		s.enviarErro(conn, req.RequestID, "Ação não reconhecida pelo servidor")
	}
}

func (s *Server) registrarRotasHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/consorcio/trechos", s.handleConsultarTrechos)
	mux.HandleFunc("/v1/search", s.handlePeerSearch)
	mux.HandleFunc("/v1/transactions/prepare", s.handlePeerPrepare)
	mux.HandleFunc("/v1/transactions/commit", s.handlePeerCommit)
	mux.HandleFunc("/v1/transactions/abort", s.handlePeerAbort)
	mux.HandleFunc("/v1/transactions/prepare-cancel", s.handlePeerPrepareCancel)
	mux.HandleFunc("/v1/transactions/commit-cancel", s.handlePeerCommitCancel)
	mux.HandleFunc("/v1/transactions/abort-cancel", s.handlePeerAbortCancel)
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
					SectionID: protocol.ComporSectionID(trecho.IdSection, s.idServidor),
					Origem:    trecho.Origem,
					Destino:   trecho.Destino,
					Assentos:  trecho.Assentos,
					Preco:     trecho.Preco,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resposta)
}

func (s *Server) handleCadastrarUser(conn net.Conn, req *protocol.Request) {
	var payload protocol.CadastroPassageiro
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de cadastro inválido")
		return
	}
	if payload.Nome == "" || payload.Email == "" || payload.Senha == "" {
		s.enviarErro(conn, req.RequestID, "Nome, email e senha são obrigatórios")
		return
	}

	usuario := domain.Usuario{Nome: payload.Nome, Email: payload.Email, Senha: payload.Senha}
	if !s.repoPassageiros.Cadastrar(usuario) {
		s.enviarErro(conn, req.RequestID, "Já existe um passageiro cadastrado com este email")
		return
	}
	s.enviarSucesso(conn, req.RequestID, "Passageiro cadastrado com sucesso! Faça login para continuar.", nil)
}

func (s *Server) handleCadastrarMotorista(conn net.Conn, req *protocol.Request) {
	var payload protocol.CadastroMotorista
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de cadastro inválido")
		return
	}
	if payload.Nome == "" || payload.Email == "" || payload.Senha == "" || payload.CNH == "" {
		s.enviarErro(conn, req.RequestID, "Nome, email, senha e CNH são obrigatórios")
		return
	}

	usuario := domain.Usuario{
		Nome: payload.Nome, Email: payload.Email, Senha: payload.Senha, CNH: payload.CNH,
	}
	if !s.repoMotoristas.Cadastrar(usuario) {
		s.enviarErro(conn, req.RequestID, "Já existe um motorista cadastrado com este email")
		return
	}
	s.enviarSucesso(conn, req.RequestID, "Motorista cadastrado com sucesso! Faça login para continuar.", nil)
}

func (s *Server) handleAuthUser(conn net.Conn, req *protocol.Request) {
	var payload protocol.LoginRequest
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de login inválido")
		return
	}
	usuario, ok := s.repoPassageiros.ValidarLogin(payload.Email, payload.Senha)
	if !ok {
		s.enviarErro(conn, req.RequestID, "Email ou senha inválidos")
		return
	}
	s.enviarSucesso(conn, req.RequestID, "Passageiro autenticado com sucesso!", protocol.LoginResult{Nome: usuario.Nome})
}

func (s *Server) handleAuthConductor(conn net.Conn, req *protocol.Request) {
	var payload protocol.LoginRequest
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de login inválido")
		return
	}
	usuario, ok := s.repoMotoristas.ValidarLogin(payload.Email, payload.Senha)
	if !ok {
		s.enviarErro(conn, req.RequestID, "Email ou senha inválidos")
		return
	}
	s.enviarSucesso(conn, req.RequestID, "Motorista autenticado com sucesso!", protocol.LoginResult{Nome: usuario.Nome})
}

func (s *Server) handlePublishRide(conn net.Conn, req *protocol.Request) {
	var payload protocol.PushRide
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de publicação inválido")
		return
	}

	rideID := "ride-" + req.RequestID
	if err := s.grafo.AddRide(rideID, payload.Route, payload.Assentos, payload.Precos, payload.DriverEmail); err != nil {
		s.enviarErro(conn, req.RequestID, err.Error())
		return
	}
	if err := s.salvarEstado(); err != nil {
		s.enviarErro(conn, req.RequestID, "Carona criada, mas não foi possível salvar o backup: "+err.Error())
		return
	}
	s.enviarSucesso(conn, req.RequestID, "Carona publicada com sucesso!", protocol.PublishRideResult{RideID: rideID})
}

func (s *Server) handleCancelRide(conn net.Conn, req *protocol.Request) {
	var payload protocol.GetID
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload inválido")
		return
	}
	if err := s.grafo.RemoveRide(payload.RideID); err != nil {
		s.enviarErro(conn, req.RequestID, err.Error())
		return
	}
	if err := s.salvarEstado(); err != nil {
		s.enviarErro(conn, req.RequestID, "Carona removida, mas não foi possível salvar o backup: "+err.Error())
		return
	}
	s.enviarSucesso(conn, req.RequestID, "Carona removida com sucesso!", nil)
}

func (s *Server) handleGetMyRides(conn net.Conn, req *protocol.Request) {
	var payload protocol.GetMyRidesRequest
	if err := json.Unmarshal(req.Payload, &payload); err != nil || payload.Email == "" {
		s.enviarErro(conn, req.RequestID, "Payload inválido ou email do motorista ausente")
		return
	}
	rides := s.grafo.RidesByDriver(payload.Email)
	s.enviarSucesso(conn, req.RequestID, fmt.Sprintf("%d carona(s) encontrada(s)", len(rides)), rides)
}

func (s *Server) handleSearchRoute(conn net.Conn, req *protocol.Request) {
	var payload protocol.SearchRoute
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de busca inválido")
		return
	}

	resultados := s.grafo.SearchRoute(payload.Source, payload.Destination)

	resposta := make([]protocol.RideInfo, 0, len(resultados))
	for _, ride := range resultados {
		trechos := make([]protocol.SectionInfo, 0, len(ride.Sections))

		for _, trecho := range ride.Sections {
			trechos = append(trechos, protocol.SectionInfo{
				IdSection: trecho.IdSection,
				Origem:    trecho.Origem,
				Destino:   trecho.Destino,
				Assentos:  trecho.Assentos,
				Preco:     trecho.Preco,
				Empresa:   s.idServidor,
			})
		}

		resposta = append(resposta, protocol.RideInfo{
			IdRide:     ride.IdRide,
			Sections:   trechos,
			IdDriver:   ride.IdDriver,
			TotalPrice: ride.TotalPrice,
		})
	}

	s.enviarSucesso(conn, req.RequestID, "Itinerários locais encontrados", resposta)
}

func (s *Server) handleBookRide(conn net.Conn, req *protocol.Request) {
	var payload protocol.BookRideRequest
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de reserva inválido")
		return
	}
	if payload.RideID == "" || payload.PassengerEmail == "" {
		s.enviarErro(conn, req.RequestID, "ID do itinerário e email do passageiro são obrigatórios")
		return
	}

	sectionIDs := s.grafo.ResolveItinerary(payload.RideID)
	if len(sectionIDs) == 0 {
		s.enviarErro(conn, req.RequestID, "Itinerário sem trechos")
		return
	}

	itinerario := make([]consorcio.ItemItinerario, 0, len(sectionIDs))
	for _, id := range sectionIDs {
		// Esta busca é local. Itinerários distribuídos precisam preservar
		// o dono de cada trecho no resultado da busca (idTrecho@idServidor).
		if _, _, err := protocol.DecomporSectionID(id); err != nil {
			id = protocol.ComporSectionID(id, s.idServidor)
		}
		itinerario = append(itinerario, consorcio.ItemItinerario{SectionID: id, Assentos: 1})
	}

	reservationID, err := consorcio.CoordenarReserva(
		itinerario,
		payload.PassengerEmail,
		s.idServidor,
		s.relogio,
		s.grafo,
		s.peersRegistry,
	)
	if err != nil {
		s.enviarErro(conn, req.RequestID, err.Error())
		return
	}
	if err := s.salvarEstado(); err != nil {
		s.enviarErro(conn, req.RequestID, "Reserva processada, mas não foi possível salvar o backup: "+err.Error())
		return
	}

	s.enviarSucesso(conn, req.RequestID, "Reserva confirmada!", protocol.BookRideResult{ReservationID: reservationID})
}

func (s *Server) handleGetBookings(conn net.Conn, req *protocol.Request) {
	s.enviarSucesso(conn, req.RequestID, "Consulta de reservas ainda não implementada", nil)
}

func (s *Server) handleCancelBooking(conn net.Conn, req *protocol.Request) {
	var payload protocol.CancelBookingRequest
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		s.enviarErro(conn, req.RequestID, "Payload de cancelamento inválido")
		return
	}
	if payload.ReservationID == "" || payload.PassengerEmail == "" {
		s.enviarErro(conn, req.RequestID, "ID da reserva e email do passageiro são obrigatórios")
		return
	}

	// Não usar CancelReservation diretamente aqui: isso libera assentos
	// apenas neste servidor. O cancelamento distribuído precisa de um
	// coordenador 2PC com decisão persistida e recuperação após reinício.
	s.enviarErro(conn, req.RequestID, "Cancelamento distribuído ainda não está conectado ao coordenador 2PC")
}

func (s *Server) handlePeerSearch(w http.ResponseWriter, r *http.Request) {
	origem := r.URL.Query().Get("origem")
	destino := r.URL.Query().Get("destino")
	if origem == "" || destino == "" {
		http.Error(w, "parâmetros 'origem' e 'destino' são obrigatórios", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.grafo.SearchRoute(origem, destino))
}

func (s *Server) handlePeerPrepare(w http.ResponseWriter, r *http.Request) {
	s.handlePeerTransaction(w, r, "prepare")
}
func (s *Server) handlePeerCommit(w http.ResponseWriter, r *http.Request) {
	s.handlePeerTransaction(w, r, "commit")
}
func (s *Server) handlePeerAbort(w http.ResponseWriter, r *http.Request) {
	s.handlePeerTransaction(w, r, "abort")
}
func (s *Server) handlePeerPrepareCancel(w http.ResponseWriter, r *http.Request) {
	s.handlePeerTransaction(w, r, "prepare-cancel")
}
func (s *Server) handlePeerCommitCancel(w http.ResponseWriter, r *http.Request) {
	s.handlePeerTransaction(w, r, "commit-cancel")
}
func (s *Server) handlePeerAbortCancel(w http.ResponseWriter, r *http.Request) {
	s.handlePeerTransaction(w, r, "abort-cancel")
}

func (s *Server) handlePeerTransaction(w http.ResponseWriter, r *http.Request, action string) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}

	var resp protocol.TransactionResponse
	switch action {
	case "prepare":
		resp = consorcio.TratarPrepare(s.grafo, s.relogio, s.idServidor, req)
	case "commit":
		resp = consorcio.TratarCommit(s.grafo, s.relogio, s.idServidor, req)
	case "abort":
		resp = consorcio.TratarAbort(s.grafo, s.relogio, s.idServidor, req)
	case "prepare-cancel":
		resp = consorcio.TratarPrepareCancelamento(s.grafo, s.relogio, s.idServidor, req)
	case "commit-cancel":
		resp = consorcio.TratarCommitCancelamento(s.grafo, s.relogio, s.idServidor, req)
	case "abort-cancel":
		resp = consorcio.TratarAbortCancelamento(s.grafo, s.relogio, s.idServidor, req)
	default:
		http.Error(w, "ação transacional desconhecida", http.StatusBadRequest)
		return
	}

	// Persiste inclusive prepares e aborts: a pendência precisa sobreviver
	// a reinicializações. Se o backup falhar, não informa sucesso ao peer.
	if resp.Accepted {
		if err := s.salvarEstado(); err != nil {
			resp.Accepted = false
			resp.Reason = "não foi possível persistir o estado: " + err.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) enviarSucesso(conn net.Conn, reqID, msg string, payload interface{}) {
	var rawPayload json.RawMessage
	if payload != nil {
		rawPayload, _ = json.Marshal(payload)
	}
	s.escreverResponse(conn, protocol.Response{
		RequestID: reqID,
		Status:    "SUCCESS",
		Message:   msg,
		Payload:   rawPayload,
	})
}

func (s *Server) enviarErro(conn net.Conn, reqID, msg string) {
	s.escreverResponse(conn, protocol.Response{
		RequestID: reqID,
		Status:    "ERROR",
		Message:   msg,
	})
}

func (s *Server) escreverResponse(conn net.Conn, resp protocol.Response) {
	bytes, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = conn.Write(append(bytes, '\n'))
}
