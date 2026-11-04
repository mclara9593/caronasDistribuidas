package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mclara9593/caronasDistribuidas/internal/connection"
	"github.com/mclara9593/caronasDistribuidas/internal/protocol"
)

// lerCampo pede um valor no terminal e insiste até receber algo não vazio
func lerCampo(reader *bufio.Reader, rotulo string) string {
	for {
		fmt.Print(rotulo)
		valor, _ := reader.ReadString('\n')
		valor = strings.TrimSpace(valor)
		if valor != "" {
			return valor
		}
		fmt.Println("⚠ Campo obrigatório. Tente novamente.")
	}
}

// ============================
// PASSAGEIRO
// ============================

// CadastrarPassageiro coleta nome, email e senha e envia o cadastro ao servidor.
// Retorna true se o cadastro foi aceito.
func CadastrarPassageiro(cliente *connection.Cliente, reader *bufio.Reader) bool {
	fmt.Println("\n--- Cadastro de Passageiro ---")
	nome := lerCampo(reader, "Nome: ")
	email := lerCampo(reader, "Email: ")
	senha := lerCampo(reader, "Senha: ")

	payload := protocol.CadastroPassageiro{
		Nome:  nome,
		Email: email,
		Senha: senha,
	}

	resp, err := cliente.EnviarRequisicao("cadastrar_user", payload)
	if err != nil {
		fmt.Printf("❌ Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Printf("📩 Resposta do Servidor: [%s] %s\n", resp.Status, resp.Message)
	return resp.Status == "SUCCESS"
}

// LoginPassageiro autentica um passageiro já cadastrado.
// Se der certo, atualiza cliente.IsLogged / UsuarioLogado / NomeLogado.
func LoginPassageiro(cliente *connection.Cliente, reader *bufio.Reader) bool {
	return login(cliente, reader, "auth_user", "Passageiro")
}

// ============================
// MOTORISTA
// ============================

// CadastrarMotorista coleta nome, email, CNH e senha e envia o cadastro ao servidor.
// É o mesmo fluxo do passageiro, só que com o campo extra da CNH.
func CadastrarMotorista(cliente *connection.Cliente, reader *bufio.Reader) bool {
	fmt.Println("\n--- Cadastro de Motorista ---")
	nome := lerCampo(reader, "Nome: ")
	email := lerCampo(reader, "Email: ")
	cnh := lerCampo(reader, "CNH: ")
	senha := lerCampo(reader, "Senha: ")

	payload := protocol.CadastroMotorista{
		Nome:  nome,
		Email: email,
		Senha: senha,
		CNH:   cnh,
	}

	resp, err := cliente.EnviarRequisicao("cadastrar_motorista", payload)
	if err != nil {
		fmt.Printf("❌ Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Printf("📩 Resposta do Servidor: [%s] %s\n", resp.Status, resp.Message)
	return resp.Status == "SUCCESS"
}

// LoginMotorista autentica um motorista já cadastrado.
func LoginMotorista(cliente *connection.Cliente, reader *bufio.Reader) bool {
	return login(cliente, reader, "auth_conductor", "Motorista")
}

// ============================
// LÓGICA COMUM
// ============================

// login concentra o fluxo de autenticação usado tanto por passageiro quanto por motorista.
// A única diferença entre os dois é qual "action" é enviada ao servidor.
func login(cliente *connection.Cliente, reader *bufio.Reader, action string, papel string) bool {
	fmt.Printf("\n--- Login de %s ---\n", papel)
	email := lerCampo(reader, "Email: ")
	senha := lerCampo(reader, "Senha: ")

	payload := protocol.LoginRequest{
		Email: email,
		Senha: senha,
	}

	resp, err := cliente.EnviarRequisicao(action, payload)
	if err != nil {
		fmt.Printf("❌ Erro de comunicação: %v\n", err)
		return false
	}

	if resp.Status != "SUCCESS" {
		fmt.Printf("❌ %s\n", resp.Message)
		return false
	}

	// O servidor manda o nome de quem logou dentro do Payload
	nome := email
	if len(resp.Payload) > 0 {
		var resultado protocol.LoginResult
		if err := json.Unmarshal(resp.Payload, &resultado); err == nil && resultado.Nome != "" {
			nome = resultado.Nome
		}
	}

	cliente.IsLogged = true
	cliente.UsuarioLogado = email
	cliente.NomeLogado = nome

	fmt.Printf("✅ Login efetuado com sucesso! Bem-vindo(a), %s.\n", nome)
	return true
}
