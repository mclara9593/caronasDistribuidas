package consorcio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/mclara9593/caronasDistribuidas/internal/graph"
)

// PeersRegistry mantém o mapeamento entre o id de cada servidor do
// consórcio e a URL base onde a sua API REST pode ser alcançada.
// É carregado uma vez na inicialização do servidor (a partir de um
// arquivo de configuração ou de flags) e consultado concorrentemente
// durante a coordenação de transações distribuídas.
type PeersRegistry struct {
	mu    sync.RWMutex
	peers map[string]string // idServidor -> URL base REST
}

// NovoPeersRegistry cria um registro já populado a partir de um mapa
// inicial de servidores conhecidos do consórcio.
func NovoPeersRegistry(peersIniciais map[string]string) PeersRegistry {
	return PeersRegistry{peers: peersIniciais}
}

// URLDoServidor retorna a URL base REST associada a um id de
// servidor, e um booleano indicando se esse servidor é conhecido
// neste registro.
func (p *PeersRegistry) URLDoServidor(idServidor string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	url, existe := p.peers[idServidor]
	return url, existe
}

// ConsultarTrechosRemotos pergunta a um servidor específico do
// consórcio (identificado pela sua URL base REST) quais trechos
// locais ele tem disponíveis para a origem/destino informados. O
// timeout curto garante que, se aquele servidor estiver fora do ar,
// a busca distribuída não fique travada esperando por ele.
func ConsultarTrechosRemotos(urlBaseServidor, origem, destino string) ([]*graph.Section, error) {
	cliente := http.Client{Timeout: 3 * time.Second}

	params := url.Values{}
	params.Set("origem", origem)
	params.Set("destino", destino)

	resp, err := cliente.Get(fmt.Sprintf("%s/consorcio/trechos?%s", urlBaseServidor, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("servidor %s não respondeu: %v", urlBaseServidor, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("servidor %s retornou status %d", urlBaseServidor, resp.StatusCode)
	}

	var trechos []*graph.Section
	if err := json.NewDecoder(resp.Body).Decode(&trechos); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta de %s: %v", urlBaseServidor, err)
	}

	return trechos, nil
}
