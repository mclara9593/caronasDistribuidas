package graph

import (
	"sync"
)

type Section struct {
	IdSection string  `json:"id"`
	Origem    string  `json:"origem"`
	Destino   string  `json:"destino"`
	Assentos  int     `json:"assentos"`
	Preco     float64 `json:"preco"`
}

// Estrutura que representa uma carona, que pode ser composta por múltiplos trechos
type Ride struct {
	IdRide     string    `json:"id"`
	Sections   []Section `json:"Sections"`
	IdDriver   string    `json:"id_driver"`
	TotalPrice float64   `json:"total_price"`
}

// Grafo representa a rede de viagens e trechos
type Grafo struct {
	mu          sync.RWMutex
	Nodes       map[string][]Section
	Rides       map[string]Ride
	Itineraries map[string][]string // id da rota -> lista de ids de trecho que ela engloba
	itinSeq     int
}

func NovoGrafo() *Grafo {
	return &Grafo{
		Nodes:       make(map[string][]Section),
		Rides:       make(map[string]Ride),
		Itineraries: make(map[string][]string),
	}
}

//==============================MANIPULAÇÃO DE CARONAS E TRECHOS======================================

// Quebra a rota do motorista em trechos e insere no Grafo
// AddRide agora recebe um preço por trecho (precos[i] corresponde ao trecho route[i] -> route[i+1]),
// em vez de um preço único aplicado a toda a rota.
func (g *Grafo) AddRide(idRide string, route []string, seats int, precos []float64, idDriver string) error {

}

// SearchRoute faz a busca BFS e retorna as opções de viagens (Rides) encontradas

func (g *Grafo) SearchRoute(origem, destino string) []Ride {

}

// registrarItinerario cria um ID único para uma combinação de trechos (uma rota
// encontrada pela busca) e guarda o mapeamento id -> lista de ids de trecho.
// Pressupõe que o chamador já está segurando o Lock de escrita.
func (g *Grafo) registrarItinerario(sections []Section) string {

}

// ResolveItinerary traduz um ID de rota (gerado pela busca) na lista de ids de
// trecho que ela contém. Se o ID não for um itinerário registrado (ex: quando
// o motorista informa direto o id de um único trecho), assume que é o próprio
// id de um trecho isolado.
func (g *Grafo) ResolveItinerary(id string) []string {

}

// RidesByDriver retorna todas as caronas publicadas por um motorista específico (filtra pelo IdDriver)
func (g *Grafo) RidesByDriver(idDriver string) []Ride {

}

// RemoveRide cancela a carona informada e limpa as arestas do Grafo
func (g *Grafo) RemoveRide(idRide string) {

}

// AdjustSeats decrementa ou incrementa assentos de forma atômica
// AdjustSeats decrementa ou incrementa assentos de forma atômica
func (g *Grafo) AdjustSeats(sectionIDs []string, delta int) bool {

}

// EstadoSerializavel representa o conteúdo do grafo pronto para ser
// salvo em disco (backup local) ou enviado como resposta a outro
// servidor do consórcio via REST, sem expor o mutex interno do Grafo.
type EstadoSerializavel struct {
	Nodes map[string][]*Section `json:"nodes"`
	Rides map[string]*Ride      `json:"rides"`
}

// Exportar retorna uma cópia do estado atual do grafo, protegida por
// leitura concorrente segura (RLock), usada tanto para gerar o backup
// local quanto para responder consultas de outros servidores.
func (g *Grafo) Exportar() EstadoSerializavel {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return EstadoSerializavel{
		Nodes: g.Nodes,
		Rides: g.Rides,
	}
}

// Importar substitui o estado do grafo pelos dados fornecidos.
// Usado exclusivamente na recuperação a partir do backup local deste
// mesmo servidor, nunca a partir de dados vindos de outro servidor.
func (g *Grafo) Importar(estado EstadoSerializavel) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Nodes = estado.Nodes
	g.Rides = estado.Rides
}
