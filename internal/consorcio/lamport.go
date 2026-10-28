package consorcio

import "sync"

// RelogioLogico implementa um relógio de Lamport simples:
// cada evento local incrementa o contador, e ao receber uma
// mensagem de outro servidor, o relógio avança para o maior
// valor entre o próprio e o recebido, mais um.
type RelogioLogico struct {
	mu        sync.Mutex
	contador  uint64
	idServico string // usado como critério de desempate em caso de timestamps iguais
}

func NovoRelogio(idServico string) *RelogioLogico {
	return &RelogioLogico{idServico: idServico}
}

// Tick incrementa o relógio para um evento local (ex: enviar uma
// nova requisição de reserva) e retorna o novo valor.
func (r *RelogioLogico) Tick() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contador++
	return r.contador
}

// Observar ajusta o relógio ao receber uma mensagem de outro
// servidor, seguindo a regra de Lamport: max(local, recebido) + 1.
func (r *RelogioLogico) Observar(recebido uint64) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if recebido > r.contador {
		r.contador = recebido
	}
	r.contador++
	return r.contador
}
