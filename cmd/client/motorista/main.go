package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mclara9593/caronasDistribuidas/cmd/client/auth"
	"github.com/mclara9593/caronasDistribuidas/internal/connection"
	"github.com/mclara9593/caronasDistribuidas/internal/protocol"
)

func publicarCarona(cliente *connection.Cliente, reader *bufio.Reader) {
	// 1. Coleta a rota completa (ex: Feira de Santana, Salvador, Aracaju),
	// repetindo a pergunta até o motorista informar pelo menos 2 cidades.
	var rota []string
	for {
		fmt.Print("Digite as cidades da rota (separadas por vírgula): ")
		rotaInput, _ := reader.ReadString('\n')
		rotaInput = strings.TrimSpace(rotaInput)

		cidadesRaw := strings.Split(rotaInput, ",")
		rota = nil
		for _, c := range cidadesRaw {
			cidadeTratada := strings.TrimSpace(c)
			if cidadeTratada != "" {
				rota = append(rota, cidadeTratada)
			}
		}

		if len(rota) < 2 {
			fmt.Println(" A rota precisa ter pelo menos 2 cidades (origem e destino).")
			continue
		}
		break
	}

	// Quebra a rota em trechos de 2 em 2 (origem -> destino) e pede um preço
	// individual para cada um, em vez de um preço único pra rota inteira.
	precos := make([]float64, len(rota)-1)
	fmt.Println("\nAgora informe o preço de cada trecho:")
	for i := 0; i < len(rota)-1; i++ {
		origem := rota[i]
		destino := rota[i+1]

		for {
			fmt.Printf("Trecho %d: %s → %s | Preço (R$): ", i+1, origem, destino)
			precoInput, _ := reader.ReadString('\n')
			precoInput = strings.TrimSpace(precoInput)
			precoInput = strings.Replace(precoInput, ",", ".", 1) // aceita vírgula como separador decimal

			valor, err := strconv.ParseFloat(precoInput, 64)
			if err != nil || valor <= 0 {
				fmt.Println(" Digite um valor numérico maior que zero (ex: 25.50).")
				continue
			}
			precos[i] = valor
			break
		}
	}

	// Pede a quantidade de assentos disponíveis individualmente para cada trecho.
	assentos := make([]int, len(rota)-1)
	fmt.Println("\nAgora informe a quantidade de assentos de cada trecho:")
	for i := 0; i < len(rota)-1; i++ {
		origem := rota[i]
		destino := rota[i+1]

		for {
			fmt.Printf("Trecho %d: %s → %s | Assentos disponíveis: ", i+1, origem, destino)
			assentosInput, _ := reader.ReadString('\n')
			assentosInput = strings.TrimSpace(assentosInput)

			valor, err := strconv.Atoi(assentosInput)
			if err != nil || valor <= 0 {
				fmt.Println(" Digite um número inteiro maior que zero.")
				continue
			}
			assentos[i] = valor
			break
		}
	}

	// Coleta a data/horário de partida
	fmt.Print("\nHorário de partida (ex: 2026-09-12 18:00): ")
	depTime, _ := reader.ReadString('\n')
	depTime = strings.TrimSpace(depTime)

	// Monta o payload conforme a struct PushRide.
	// DriverEmail é o email de quem está logado — é isso que permite depois
	// filtrar "minhas caronas" corretamente.
	payload := protocol.PushRide{
		Route:         rota,
		DepartureTime: depTime,
		Assentos:      assentos,
		Precos:        precos,
		DriverEmail:   cliente.UsuarioLogado,
	}

	// Envia a requisição via socket
	resp, err := cliente.EnviarRequisicao("publish_ride", payload)
	if err != nil {
		fmt.Printf("Erro ao publicar carona: %v\n", err)
		return
	}

	fmt.Printf("📩 Resposta do Servidor: [%s] %s\n", resp.Status, resp.Message)

	// Se deu certo, o servidor manda o ID da carona dentro do Payload.
	// Repara no "null": se o payload vier vazio, o JSON manda literalmente a palavra
	// null (4 bytes) em vez de nada — por isso não basta checar len(resp.Payload) > 0.
	if resp.Status == "SUCCESS" && len(resp.Payload) > 0 && string(resp.Payload) != "null" {
		var resultado protocol.PublishRideResult
		if err := json.Unmarshal(resp.Payload, &resultado); err == nil && resultado.RideID != "" {
			fmt.Printf("🚗 ID da carona: %s (guarde esse ID para cancelar depois)\n", resultado.RideID)
		}
	}
}

func consultarCaronas(cliente *connection.Cliente) {
	resp, err := cliente.EnviarRequisicao("get_my_rides", protocol.GetMyRidesRequest{
		Email: cliente.UsuarioLogado,
	})
	if err != nil {
		fmt.Printf("Erro ao consultar caronas: %v\n", err)
		return
	}

	fmt.Printf("📩 Resposta do Servidor: [%s] %s\n", resp.Status, resp.Message)

	if resp.Status != "SUCCESS" || len(resp.Payload) == 0 {
		return
	}

	var caronas []protocol.RideInfo
	if err := json.Unmarshal(resp.Payload, &caronas); err != nil {
		fmt.Printf("Erro ao interpretar a resposta: %v\n", err)
		return
	}

	if len(caronas) == 0 {
		fmt.Println("Você ainda não publicou nenhuma carona.")
		return
	}

	for _, carona := range caronas {
		fmt.Printf("\n🚗 Carona %s — total R$ %.2f\n", carona.IdRide, carona.TotalPrice)
		for _, trecho := range carona.Sections {
			fmt.Printf("   %s → %s | %d vaga(s) | R$ %.2f | id: %s\n",
				trecho.Origem, trecho.Destino, trecho.Assentos, trecho.Preco, trecho.IdSection)
		}
	}
}

func cancelarCarona(cliente *connection.Cliente, reader *bufio.Reader) {
	fmt.Print("Digite o ID da carona que deseja cancelar: ")
	idCarona, _ := reader.ReadString('\n')
	idCarona = strings.TrimSpace(idCarona)

	fmt.Printf("Tem certeza que deseja cancelar a carona %s? (s/n): ", idCarona)
	confirmacao, _ := reader.ReadString('\n')
	confirmacao = strings.ToLower(strings.TrimSpace(confirmacao))

	if confirmacao != "s" && confirmacao != "sim" {
		fmt.Println("Cancelamento não realizado.")
		return
	}

	resp, err := cliente.EnviarRequisicao(
		"cancel_ride",
		protocol.GetID{RideID: idCarona},
	)
	if err != nil {
		fmt.Printf("Erro ao cancelar carona: %v\n", err)
		return
	}

	fmt.Printf("📩 Resposta do Servidor: [%s] %s\n", resp.Status, resp.Message)
}

func main() {
	empresaFlag := flag.String("empresa", "", "Identificador da empresa do consórcio a quem conectar (A, B ou C). Se omitido, a escolha é feita de forma interativa.")
	addrFlag := flag.String("addr", "", "Endereço TCP do servidor (ex: 192.168.1.50:9593). Se omitido, usa o endereço padrão da empresa.")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)

	var idEmpresa, endereco string
	var err error

	if *empresaFlag != "" {
		idEmpresa = strings.ToUpper(*empresaFlag)
		endPadrao, existe := connection.ServidoresConsorcio[idEmpresa]
		if !existe && *addrFlag == "" {
			fmt.Printf("Empresa %q não reconhecida.\n", idEmpresa)
			os.Exit(1)
		}
		if *addrFlag != "" {
			endereco = *addrFlag
		} else {
			endereco = endPadrao
		}
	} else if *addrFlag != "" {
		// Endereço passado mas sem flag -empresa: pergunta a empresa correspondente
		fmt.Print("Digite a letra da empresa deste servidor (A, B ou C): ")
		escolha, _ := reader.ReadString('\n')
		idEmpresa = strings.ToUpper(strings.TrimSpace(escolha))
		if idEmpresa == "" {
			idEmpresa = "A"
		}
		endereco = *addrFlag
	} else {
		idEmpresa, endereco, err = connection.EscolherServidor(reader)
		if err != nil {
			fmt.Printf("Erro: %v\n", err)
			os.Exit(1)
		}
	}

	conn, err := connection.ConectarAoServidor(endereco, 5)
	if err != nil {
		fmt.Printf(" Erro ao conectar: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	cliente := connection.NewCliente("Motorista", idEmpresa, conn)
	fmt.Printf("Conectado com sucesso ao servidor da empresa %s em %s!\n", idEmpresa, endereco)

	for {
		fmt.Println("\n--- VAIJUNTO: PAINEL DO MOTORISTA ---")
		fmt.Println("1. Cadastrar (1º acesso)")
		fmt.Println("2. Login")
		fmt.Println("3. Publicar Carona")
		fmt.Println("4. Consultar Minhas Caronas e Passageiros")
		fmt.Println("5. Cancelar Carona")
		fmt.Println("6. Sair")
		fmt.Print("Escolha uma opção: ")

		opcao, _ := reader.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		precisaLogin := opcao == "3" || opcao == "4" || opcao == "5"
		if precisaLogin && !cliente.IsLogged {
			fmt.Println("\n🔒 Acesso negado! Você precisa fazer login (Opção 2) antes de realizar ações.")
			continue
		}

		switch opcao {
		case "1":
			if cliente.IsLogged {
				fmt.Printf("Você já está logado como %s!\n", cliente.NomeLogado)
				continue
			}
			auth.CadastrarMotorista(cliente, reader)
		case "2":
			if cliente.IsLogged {
				fmt.Printf("Você já está logado como %s!\n", cliente.NomeLogado)
				continue
			}
			auth.LoginMotorista(cliente, reader)
		case "3":
			publicarCarona(cliente, reader)
		case "4":
			consultarCaronas(cliente)
		case "5":
			cancelarCarona(cliente, reader)
		case "6":
			fmt.Println("Encerrando conexão do motorista...")
			return
		default:
			fmt.Println("Opção inválida. Tente novamente.")
		}
	}
}
