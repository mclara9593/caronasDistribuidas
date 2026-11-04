package domain

import "sync"

// Usuario representa um passageiro ou motorista cadastrado no sistema.
type Usuario struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
	CNH   string `json:"cnh,omitempty"` // apenas motoristas possuem CNH
}

// Repositorio é um armazenamento em memória de usuários, indexado por email.
// Thread-safe para uso concorrente entre goroutines do servidor.
type Repositorio struct {
	mu       sync.RWMutex
	usuarios map[string]Usuario // email -> Usuario
}

// NovoRepositorio cria um repositório vazio pronto para uso.
func NovoRepositorio() *Repositorio {
	return &Repositorio{
		usuarios: make(map[string]Usuario),
	}
}

// Cadastrar registra um novo usuário. Retorna false se o email já existir.
func (r *Repositorio) Cadastrar(u Usuario) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, existe := r.usuarios[u.Email]; existe {
		return false
	}
	r.usuarios[u.Email] = u
	return true
}

// ValidarLogin verifica email e senha. Retorna o usuário e true se ok.
func (r *Repositorio) ValidarLogin(email, senha string) (Usuario, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, existe := r.usuarios[email]
	if !existe || u.Senha != senha {
		return Usuario{}, false
	}
	return u, true
}
