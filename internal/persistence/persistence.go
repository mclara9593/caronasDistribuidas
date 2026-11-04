package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mclara9593/caronasDistribuidas/internal/graph"
	"github.com/mclara9593/caronasDistribuidas/internal/protocol"
)

// CaminhoBackup monta o nome do arquivo de backup a partir do
// identificador da própria empresa, garantindo que cada instância
// de servidor grave em um arquivo próprio, isolado dos demais.
func CaminhoBackup(idEmpresa string) string {
	return filepath.Join("backups", fmt.Sprintf("estado_%s.json", idEmpresa))
}

// SalvarEstado grava o estado atual do grafo em disco de forma
// atômica: escreve primeiro num arquivo temporário e só depois
// renomeia para o nome final, evitando que uma queda do servidor
// no meio da escrita deixe o backup corrompido pela metade.
func SalvarEstado(idEmpresa string, g *graph.Grafo) error {
	estado := g.Exportar()

	dados, err := json.MarshalIndent(estado, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar estado: %v", err)
	}

	caminhoFinal := CaminhoBackup(idEmpresa)
	if err := os.MkdirAll(filepath.Dir(caminhoFinal), 0o755); err != nil {
		return fmt.Errorf("erro ao criar diretório de backup: %v", err)
	}

	arquivoTemp := caminhoFinal + ".tmp"
	if err := os.WriteFile(arquivoTemp, dados, 0o644); err != nil {
		return fmt.Errorf("erro ao escrever arquivo temporário: %v", err)
	}

	return os.Rename(arquivoTemp, caminhoFinal)
}

// CarregarEstado lê o arquivo de backup local desta empresa, se
// existir, e restaura o grafo com os dados salvos antes da última
// queda ou desligamento. Se o arquivo ainda não existir (primeira
// execução do servidor), retorna sem erro e o grafo começa vazio.
func CarregarEstado(idEmpresa string, g *graph.Grafo) error {
	caminho := CaminhoBackup(idEmpresa)

	dados, err := os.ReadFile(caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("erro ao ler backup: %v", err)
	}

	var estado graph.EstadoSerializavel
	if err := json.Unmarshal(dados, &estado); err != nil {
		return fmt.Errorf("erro ao decodificar backup: %v", err)
	}

	g.Importar(estado)
	return nil
}

type EstadoCoordenador struct {
	TransactionID  string                                   `json:"transaction_id"`
	PassengerEmail string                                   `json:"passenger_email"`
	OriginServer   string                                   `json:"origin_server"`
	Timestamp      uint64                                   `json:"timestamp"`
	Decision       string                                   `json:"decision"`  // "commit" ou "abort"
	Operation      string                                   `json:"operation"` // "booking" ou "cancellation"
	Sections       map[string][]protocol.TransactionSection `json:"sections"`
}

func caminhoCoordenador(idServidor string) string {
	return filepath.Join("backups", fmt.Sprintf("coordenador_%s.json", idServidor))
}

func SalvarEstadoCoordenador(idServidor string, estado EstadoCoordenador) error {
	dados, err := json.MarshalIndent(estado, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar estado do coordenador: %w", err)
	}
	caminho := caminhoCoordenador(idServidor)
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return fmt.Errorf("criar diretório do coordenador: %w", err)
	}

	tmp := caminho + ".tmp"
	if err := os.WriteFile(tmp, dados, 0o600); err != nil {
		return fmt.Errorf("escrever estado temporário do coordenador: %w", err)
	}
	if err := os.Rename(tmp, caminho); err != nil {
		return fmt.Errorf("substituir estado do coordenador: %w", err)
	}
	return nil
}

func CarregarEstadoCoordenador(idServidor string) (*EstadoCoordenador, error) {
	dados, err := os.ReadFile(caminhoCoordenador(idServidor))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ler estado do coordenador: %w", err)
	}

	var estado EstadoCoordenador
	if err := json.Unmarshal(dados, &estado); err != nil {
		return nil, fmt.Errorf("decodificar estado do coordenador: %w", err)
	}
	return &estado, nil
}

func LimparEstadoCoordenador(idServidor string) error {
	err := os.Remove(caminhoCoordenador(idServidor))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remover estado do coordenador: %w", err)
	}
	return nil
}
