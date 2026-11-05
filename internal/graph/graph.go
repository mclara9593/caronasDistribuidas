package graph

import (
	"fmt"
	"sync"
)

type Section struct {
	IdSection string  `json:"id"`
	Origem    string  `json:"origem"`
	Destino   string  `json:"destino"`
	Assentos  int     `json:"assentos"`
	Preco     float64 `json:"preco"`
}

type Ride struct {
	IdRide     string    `json:"id"`
	Sections   []Section `json:"Sections"`
	IdDriver   string    `json:"id_driver"`
	TotalPrice float64   `json:"total_price"`
}

type Reservation struct {
	ReservationID  string   `json:"reservation_id"`
	PassengerEmail string   `json:"passenger_email"`
	SectionIDs     []string `json:"section_ids"`
}

type PendingTransaction struct {
	TransactionID  string   `json:"transaction_id"`
	PassengerEmail string   `json:"passenger_email"`
	SectionIDs     []string `json:"section_ids"`
}

type PendingCancellation struct {
	ReservationID  string   `json:"reservation_id"`
	PassengerEmail string   `json:"passenger_email"`
	SectionIDs     []string `json:"section_ids"`
}

type Grafo struct {
	mu sync.RWMutex

	Nodes       map[string][]Section
	Rides       map[string]Ride
	Itineraries map[string][]string
	itinSeq     int

	Pendentes              map[string]PendingTransaction
	Reservas               map[string]Reservation
	CancelamentosPendentes map[string]PendingCancellation
}

func NovoGrafo() *Grafo {
	return &Grafo{
		Nodes:                  make(map[string][]Section),
		Rides:                  make(map[string]Ride),
		Itineraries:            make(map[string][]string),
		Pendentes:              make(map[string]PendingTransaction),
		Reservas:               make(map[string]Reservation),
		CancelamentosPendentes: make(map[string]PendingCancellation),
	}
}

// AddRide divide a rota em trechos consecutivos e adiciona a carona ao grafo.
func (g *Grafo) AddRide(
	idRide string,
	route []string,
	seats []int,
	precos []float64,
	idDriver string,
) error {
	if idRide == "" || idDriver == "" {
		return fmt.Errorf("ID da carona e email do motorista são obrigatórios")
	}
	if len(route) < 2 {
		return fmt.Errorf("a rota precisa ter pelo menos duas cidades")
	}
	if len(seats) != len(route)-1 {
		return fmt.Errorf(
			"esperava %d quantidade(s) de assentos, recebeu %d",
			len(route)-1,
			len(seats),
		)
	}
	if len(precos) != len(route)-1 {
		return fmt.Errorf(
			"esperava %d preço(s), recebeu %d",
			len(route)-1,
			len(precos),
		)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, existe := g.Rides[idRide]; existe {
		return fmt.Errorf("a carona %q já existe", idRide)
	}

	rideSections := make([]Section, 0, len(route)-1)
	var totalPrice float64

	for i := 0; i < len(route)-1; i++ {
		if route[i] == "" || route[i+1] == "" {
			return fmt.Errorf("a rota contém uma cidade vazia")
		}
		if seats[i] <= 0 {
			return fmt.Errorf("a quantidade de assentos do trecho %d deve ser maior que zero", i+1)
		}
		if precos[i] <= 0 {
			return fmt.Errorf("o preço do trecho %d deve ser maior que zero", i+1)
		}

		section := Section{
			IdSection: fmt.Sprintf("%s-sec-%s-%s", idRide, route[i], route[i+1]),
			Origem:    route[i],
			Destino:   route[i+1],
			Assentos:  seats[i],
			Preco:     precos[i],
		}

		rideSections = append(rideSections, section)
		g.Nodes[section.Origem] = append(g.Nodes[section.Origem], section)
		totalPrice += section.Preco
	}

	g.Rides[idRide] = Ride{
		IdRide:     idRide,
		Sections:   rideSections,
		IdDriver:   idDriver,
		TotalPrice: totalPrice,
	}

	return nil
}

// SearchRoute busca itinerários locais usando busca em largura.
func (g *Grafo) SearchRoute(origem, destino string) []Ride {
	g.mu.RLock()

	type pathNode struct {
		cidadeAtual string
		caminho     []Section
		visitados   map[string]bool
	}

	fila := []pathNode{{
		cidadeAtual: origem,
		caminho:     []Section{},
		visitados:   map[string]bool{origem: true},
	}}

	var resultados []Ride

	for len(fila) > 0 {
		atual := fila[0]
		fila = fila[1:]

		if atual.cidadeAtual == destino {
			var total float64
			for _, section := range atual.caminho {
				total += section.Preco
			}

			resultados = append(resultados, Ride{
				Sections:   append([]Section(nil), atual.caminho...),
				IdDriver:   "multi",
				TotalPrice: total,
			})
			continue
		}

		for _, section := range g.Nodes[atual.cidadeAtual] {
			if section.Assentos <= 0 || atual.visitados[section.Destino] {
				continue
			}

			visitados := make(map[string]bool, len(atual.visitados)+1)
			for cidade, visitado := range atual.visitados {
				visitados[cidade] = visitado
			}
			visitados[section.Destino] = true

			caminho := append([]Section(nil), atual.caminho...)
			caminho = append(caminho, section)

			fila = append(fila, pathNode{
				cidadeAtual: section.Destino,
				caminho:     caminho,
				visitados:   visitados,
			})
		}
	}

	g.mu.RUnlock()

	g.mu.Lock()
	for i := range resultados {
		resultados[i].IdRide = g.registrarItinerarioSemLock(resultados[i].Sections)
	}
	g.mu.Unlock()

	return resultados
}

// registrarItinerarioSemLock pressupõe que o chamador possui o lock de escrita.
func (g *Grafo) registrarItinerarioSemLock(sections []Section) string {
	g.itinSeq++
	id := fmt.Sprintf("rota-%d", g.itinSeq)

	sectionIDs := make([]string, len(sections))
	for i, section := range sections {
		sectionIDs[i] = section.IdSection
	}
	g.Itineraries[id] = sectionIDs

	return id
}

// ResolveItinerary retorna uma cópia dos IDs de trechos associados ao itinerário.
func (g *Grafo) ResolveItinerary(id string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if sectionIDs, existe := g.Itineraries[id]; existe {
		return append([]string(nil), sectionIDs...)
	}
	return []string{id}
}

func (g *Grafo) RidesByDriver(idDriver string) []Ride {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var resultado []Ride
	for _, ride := range g.Rides {
		if ride.IdDriver != idDriver {
			continue
		}

		ride.Sections = append([]Section(nil), ride.Sections...)
		resultado = append(resultado, ride)
	}
	return resultado
}

// RemoveRide mantém a assinatura usada pelo servidor.
// A remoção direta continua recusando caronas com reservas; use o fluxo
// coordenado do servidor para cancelar reservas distribuídas primeiro.
func (g *Grafo) RemoveRide(idRide string) error {
	return g.RemoveRideIfUnreserved(idRide)
}

func (g *Grafo) RemoveRideIfUnreserved(idRide string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	ride, existe := g.Rides[idRide]
	if !existe {
		return fmt.Errorf("carona %q não encontrada", idRide)
	}

	sectionIDs := make(map[string]bool, len(ride.Sections))
	for _, section := range ride.Sections {
		sectionIDs[section.IdSection] = true
	}

	for _, pending := range g.Pendentes {
		for _, id := range pending.SectionIDs {
			if sectionIDs[id] {
				return fmt.Errorf("carona possui uma reserva pendente")
			}
		}
	}

	for _, reservation := range g.Reservas {
		for _, id := range reservation.SectionIDs {
			if sectionIDs[id] {
				return fmt.Errorf("carona possui reservas confirmadas")
			}
		}
	}

	g.removerRideSemLock(idRide, ride)
	return nil
}

// ReservationsForRide retorna cópias das reservas locais que usam pelo
// menos um trecho da carona. Cada resultado contém todos os trechos locais
// daquela reserva; a coordenação reunirá os trechos guardados nos peers.
func (g *Grafo) ReservationsForRide(rideID string) ([]Reservation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ride, existe := g.Rides[rideID]
	if !existe {
		return nil, fmt.Errorf("carona %q não encontrada", rideID)
	}

	sectionIDs := make(map[string]bool, len(ride.Sections))
	for _, section := range ride.Sections {
		sectionIDs[section.IdSection] = true
	}

	var resultado []Reservation
	for _, reservation := range g.Reservas {
		usaCarona := false
		for _, id := range reservation.SectionIDs {
			if sectionIDs[id] {
				usaCarona = true
				break
			}
		}
		if !usaCarona {
			continue
		}

		reservation.SectionIDs = append([]string(nil), reservation.SectionIDs...)
		resultado = append(resultado, reservation)
	}

	return resultado, nil
}

// AdjustSeats preserva a API anterior para outros pontos do projeto.
// Reservas distribuídas devem usar PrepareReservation/CommitReservation.
func (g *Grafo) AdjustSeats(sectionIDs []string, delta int) bool {
	if len(sectionIDs) == 0 {
		return false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	seen := make(map[string]bool, len(sectionIDs))
	for _, id := range sectionIDs {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true

		section, existe := g.buscarSectionSemLock(id)
		if !existe || section.Assentos+delta < 0 {
			return false
		}
	}

	for _, id := range sectionIDs {
		g.ajustarAssentosSemLock(id, delta)
	}
	return true
}

func (g *Grafo) RideByID(rideID string) (Ride, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ride, existe := g.Rides[rideID]
	if !existe {
		return Ride{}, false
	}
	ride.Sections = append([]Section(nil), ride.Sections...)
	return ride, true
}

// PrepareReservation registra uma reserva pendente sem descontar imediatamente
// os assentos. As pendências são consideradas ao verificar a capacidade.
func (g *Grafo) PrepareReservation(
	transactionID string,
	passengerEmail string,
	sectionIDs []string,
) error {
	if transactionID == "" || passengerEmail == "" || len(sectionIDs) == 0 {
		return fmt.Errorf("transação, email e trechos são obrigatórios")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if pending, existe := g.Pendentes[transactionID]; existe {
		if pending.PassengerEmail == passengerEmail &&
			mesmosIDs(pending.SectionIDs, sectionIDs) {
			return nil
		}
		return fmt.Errorf("transação já existe com dados diferentes")
	}

	if _, existe := g.Reservas[transactionID]; existe {
		return nil
	}

	seen := make(map[string]bool, len(sectionIDs))
	for _, id := range sectionIDs {
		if id == "" || seen[id] {
			return fmt.Errorf("trecho vazio ou duplicado: %q", id)
		}
		seen[id] = true

		section, existe := g.buscarSectionSemLock(id)
		if !existe {
			return fmt.Errorf("trecho %q não encontrado", id)
		}
		if section.Assentos-g.pendentesNoTrechoSemLock(id) < 1 {
			return fmt.Errorf("não há assento disponível no trecho %q", id)
		}
	}

	g.Pendentes[transactionID] = PendingTransaction{
		TransactionID:  transactionID,
		PassengerEmail: passengerEmail,
		SectionIDs:     append([]string(nil), sectionIDs...),
	}
	return nil
}

// CommitReservation confirma a reserva, desconta um assento por trecho e
// registra a reserva usando o ID da transação como ID da reserva.
func (g *Grafo) CommitReservation(transactionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, existe := g.Reservas[transactionID]; existe {
		return nil
	}

	pending, existe := g.Pendentes[transactionID]
	if !existe {
		return fmt.Errorf("transação pendente %q não encontrada", transactionID)
	}

	for _, id := range pending.SectionIDs {
		section, existe := g.buscarSectionSemLock(id)
		if !existe || section.Assentos < 1 {
			return fmt.Errorf("assento indisponível no trecho %q", id)
		}
	}

	for _, id := range pending.SectionIDs {
		g.ajustarAssentosSemLock(id, -1)
	}

	g.Reservas[transactionID] = Reservation{
		ReservationID:  transactionID,
		PassengerEmail: pending.PassengerEmail,
		SectionIDs:     append([]string(nil), pending.SectionIDs...),
	}
	delete(g.Pendentes, transactionID)

	return nil
}

func (g *Grafo) AbortReservation(transactionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, confirmada := g.Reservas[transactionID]; confirmada {
		return fmt.Errorf("a transação %q já foi confirmada", transactionID)
	}

	delete(g.Pendentes, transactionID)
	return nil
}

// CancelReservation mantém a API anterior para cancelamento local.
func (g *Grafo) CancelReservation(
	reservationID string,
	passengerEmail string,
) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	reservation, existe := g.Reservas[reservationID]
	if !existe {
		return fmt.Errorf("reserva %q não encontrada", reservationID)
	}
	if reservation.PassengerEmail != passengerEmail {
		return fmt.Errorf("a reserva não pertence a este passageiro")
	}

	if err := g.validarTrechosDaReservaSemLock(reservation.SectionIDs); err != nil {
		return err
	}

	for _, id := range reservation.SectionIDs {
		g.ajustarAssentosSemLock(id, 1)
	}

	delete(g.Reservas, reservationID)
	delete(g.CancelamentosPendentes, reservationID)
	return nil
}

// PrepareCancellation registra a intenção de cancelar, sem liberar assentos.
func (g *Grafo) PrepareCancellation(
	reservationID string,
	passengerEmail string,
	sectionIDs []string,
) error {
	if reservationID == "" || passengerEmail == "" || len(sectionIDs) == 0 {
		return fmt.Errorf("reserva, email e trechos são obrigatórios")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	reservation, existe := g.Reservas[reservationID]
	if !existe {
		return fmt.Errorf("reserva %q não encontrada", reservationID)
	}
	if reservation.PassengerEmail != passengerEmail {
		return fmt.Errorf("a reserva não pertence a este passageiro")
	}
	if !mesmosIDs(reservation.SectionIDs, sectionIDs) {
		return fmt.Errorf("os trechos enviados não correspondem à reserva")
	}

	if pending, existe := g.CancelamentosPendentes[reservationID]; existe {
		if pending.PassengerEmail == passengerEmail &&
			mesmosIDs(pending.SectionIDs, sectionIDs) {
			return nil
		}
		return fmt.Errorf("já existe um cancelamento pendente com dados diferentes")
	}

	g.CancelamentosPendentes[reservationID] = PendingCancellation{
		ReservationID:  reservationID,
		PassengerEmail: passengerEmail,
		SectionIDs:     append([]string(nil), sectionIDs...),
	}
	return nil
}

func (g *Grafo) CommitCancellation(reservationID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	pending, existe := g.CancelamentosPendentes[reservationID]
	if !existe {
		if _, reservaAindaExiste := g.Reservas[reservationID]; !reservaAindaExiste {
			return nil
		}
		return fmt.Errorf("cancelamento pendente %q não encontrado", reservationID)
	}

	reservation, existe := g.Reservas[reservationID]
	if !existe {
		delete(g.CancelamentosPendentes, reservationID)
		return nil
	}
	if reservation.PassengerEmail != pending.PassengerEmail ||
		!mesmosIDs(reservation.SectionIDs, pending.SectionIDs) {
		return fmt.Errorf("a reserva não corresponde ao cancelamento pendente")
	}

	if err := g.validarTrechosDaReservaSemLock(reservation.SectionIDs); err != nil {
		return err
	}

	for _, id := range reservation.SectionIDs {
		g.ajustarAssentosSemLock(id, 1)
	}

	delete(g.Reservas, reservationID)
	delete(g.CancelamentosPendentes, reservationID)
	return nil
}

func (g *Grafo) AbortCancellation(
	reservationID string,
	passengerEmail string,
) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	pending, existe := g.CancelamentosPendentes[reservationID]
	if !existe {
		return nil
	}
	if pending.PassengerEmail != passengerEmail {
		return fmt.Errorf("o cancelamento pendente não pertence a este passageiro")
	}

	delete(g.CancelamentosPendentes, reservationID)
	return nil
}

type EstadoSerializavel struct {
	Nodes                  map[string][]Section           `json:"nodes"`
	Rides                  map[string]Ride                `json:"rides"`
	Itineraries            map[string][]string            `json:"itineraries"`
	ItinSeq                int                            `json:"itin_seq"`
	Pendentes              map[string]PendingTransaction  `json:"pendentes"`
	Reservas               map[string]Reservation         `json:"reservas"`
	CancelamentosPendentes map[string]PendingCancellation `json:"cancelamentos_pendentes"`
}

func (g *Grafo) Exportar() EstadoSerializavel {
	g.mu.RLock()
	defer g.mu.RUnlock()

	estado := EstadoSerializavel{
		Nodes:                  make(map[string][]Section, len(g.Nodes)),
		Rides:                  make(map[string]Ride, len(g.Rides)),
		Itineraries:            make(map[string][]string, len(g.Itineraries)),
		ItinSeq:                g.itinSeq,
		Pendentes:              make(map[string]PendingTransaction, len(g.Pendentes)),
		Reservas:               make(map[string]Reservation, len(g.Reservas)),
		CancelamentosPendentes: make(map[string]PendingCancellation, len(g.CancelamentosPendentes)),
	}

	for origem, sections := range g.Nodes {
		estado.Nodes[origem] = append([]Section(nil), sections...)
	}
	for id, ride := range g.Rides {
		ride.Sections = append([]Section(nil), ride.Sections...)
		estado.Rides[id] = ride
	}
	for id, sectionIDs := range g.Itineraries {
		estado.Itineraries[id] = append([]string(nil), sectionIDs...)
	}
	for id, pending := range g.Pendentes {
		pending.SectionIDs = append([]string(nil), pending.SectionIDs...)
		estado.Pendentes[id] = pending
	}
	for id, reservation := range g.Reservas {
		reservation.SectionIDs = append([]string(nil), reservation.SectionIDs...)
		estado.Reservas[id] = reservation
	}
	for id, pending := range g.CancelamentosPendentes {
		pending.SectionIDs = append([]string(nil), pending.SectionIDs...)
		estado.CancelamentosPendentes[id] = pending
	}

	return estado
}

func (g *Grafo) Importar(estado EstadoSerializavel) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Nodes = make(map[string][]Section, len(estado.Nodes))
	for origem, sections := range estado.Nodes {
		g.Nodes[origem] = append([]Section(nil), sections...)
	}

	g.Rides = make(map[string]Ride, len(estado.Rides))
	for id, ride := range estado.Rides {
		ride.Sections = append([]Section(nil), ride.Sections...)
		g.Rides[id] = ride
	}

	g.Itineraries = make(map[string][]string, len(estado.Itineraries))
	for id, sectionIDs := range estado.Itineraries {
		g.Itineraries[id] = append([]string(nil), sectionIDs...)
	}

	g.Pendentes = make(map[string]PendingTransaction, len(estado.Pendentes))
	for id, pending := range estado.Pendentes {
		pending.SectionIDs = append([]string(nil), pending.SectionIDs...)
		g.Pendentes[id] = pending
	}

	g.Reservas = make(map[string]Reservation, len(estado.Reservas))
	for id, reservation := range estado.Reservas {
		reservation.SectionIDs = append([]string(nil), reservation.SectionIDs...)
		g.Reservas[id] = reservation
	}

	g.CancelamentosPendentes = make(
		map[string]PendingCancellation,
		len(estado.CancelamentosPendentes),
	)
	for id, pending := range estado.CancelamentosPendentes {
		pending.SectionIDs = append([]string(nil), pending.SectionIDs...)
		g.CancelamentosPendentes[id] = pending
	}

	g.itinSeq = estado.ItinSeq
}

func (g *Grafo) buscarSectionSemLock(id string) (Section, bool) {
	for _, sections := range g.Nodes {
		for _, section := range sections {
			if section.IdSection == id {
				return section, true
			}
		}
	}
	return Section{}, false
}

func (g *Grafo) pendentesNoTrechoSemLock(id string) int {
	total := 0
	for _, pending := range g.Pendentes {
		for _, sectionID := range pending.SectionIDs {
			if sectionID == id {
				total++
			}
		}
	}
	return total
}

func (g *Grafo) ajustarAssentosSemLock(id string, delta int) {
	for origem, sections := range g.Nodes {
		for i := range sections {
			if sections[i].IdSection == id {
				g.Nodes[origem][i].Assentos += delta
			}
		}
	}

	for rideID, ride := range g.Rides {
		for i := range ride.Sections {
			if ride.Sections[i].IdSection == id {
				ride.Sections[i].Assentos += delta
			}
		}
		g.Rides[rideID] = ride
	}
}

func (g *Grafo) removerRideSemLock(rideID string, ride Ride) {
	for _, section := range ride.Sections {
		sections := g.Nodes[section.Origem]
		atualizados := make([]Section, 0, len(sections))

		for _, atual := range sections {
			if atual.IdSection != section.IdSection {
				atualizados = append(atualizados, atual)
			}
		}

		if len(atualizados) == 0 {
			delete(g.Nodes, section.Origem)
		} else {
			g.Nodes[section.Origem] = atualizados
		}
	}

	delete(g.Rides, rideID)
}

func (g *Grafo) validarTrechosDaReservaSemLock(sectionIDs []string) error {
	for _, id := range sectionIDs {
		if _, existe := g.buscarSectionSemLock(id); !existe {
			return fmt.Errorf("trecho %q da reserva não existe mais", id)
		}
	}
	return nil
}

func mesmosIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (g *Grafo) ReservationsByPassenger(passengerEmail string) []Reservation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var resultado []Reservation
	for _, reservation := range g.Reservas {
		if reservation.PassengerEmail != passengerEmail {
			continue
		}

		reservation.SectionIDs = append([]string(nil), reservation.SectionIDs...)
		resultado = append(resultado, reservation)
	}
	return resultado
}

// ReservationsUsingSections retorna reservas locais que usam pelo menos
// um dos trechos informados. As listas retornadas são cópias.
func (g *Grafo) ReservationsUsingSections(sectionIDs []string) []Reservation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	alvo := make(map[string]bool, len(sectionIDs))
	for _, id := range sectionIDs {
		if id != "" {
			alvo[id] = true
		}
	}

	var resultado []Reservation
	for _, reservation := range g.Reservas {
		usaTrecho := false
		for _, id := range reservation.SectionIDs {
			if alvo[id] {
				usaTrecho = true
				break
			}
		}
		if !usaTrecho {
			continue
		}

		reservation.SectionIDs = append([]string(nil), reservation.SectionIDs...)
		resultado = append(resultado, reservation)
	}
	return resultado
}
