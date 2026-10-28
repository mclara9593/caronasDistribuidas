package consorcio

import (
	"fmt"
	"sync"

	"github.com/mclara9593/caronas/internal/graph"
	"github.com/mclara9593/caronas/internal/protocol"
)

// ItemItinerario identifica um trecho de um itinerário distribuído.
// O id do servidor dono já vem embutido no próprio SectionID
// (formato "idTrecho@idServidor"), sem precisar de um campo extra.
type ItemItinerario struct {
	SectionID string
	Assentos  int
}

func CoordenarReserva(itinerario []ItemItinerario, passageiroID, meuIDServidor string, relogio *RelogioLogico, grafo *graph.Grafo, peers PeersRegistry) (string, error) {
	transacaoID := gerarTransacaoID(meuIDServidor)
	timestamp := relogio.Tick()

	var wg sync.WaitGroup
	respostas := make([]bool, len(itinerario))
	erros := make([]error, len(itinerario))

	for i, item := range itinerario {
		wg.Add(1)
		go func(i int, item ItemItinerario) {
			defer wg.Done()

			_, idServidorDono, err := protocol.DecomporSectionID(item.SectionID)
			if err != nil {
				erros[i] = err
				return
			}

			if idServidorDono == meuIDServidor {
				ok, err := prepararLocal(grafo, transacaoID, item, passageiroID, timestamp)
				respostas[i], erros[i] = ok, err
				return
			}

			urlBase, existe := peers.URLDoServidor(idServidorDono)
			if !existe {
				erros[i] = fmt.Errorf("servidor %s desconhecido no consórcio", idServidorDono)
				return
			}

			ok, err := prepararRemoto(urlBase, transacaoID, item, passageiroID, timestamp)
			respostas[i], erros[i] = ok, err
		}(i, item)
	}
	wg.Wait()

	// ... resto igual ao que já te mostrei (checar respostas, confirmar ou abortar)
}
