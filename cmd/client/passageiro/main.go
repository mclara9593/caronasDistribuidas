package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mclara9593/caronasDistribuidas/cmd/client/auth"
	"github.com/mclara9593/caronasDistribuidas/internal/connection"
	"github.com/mclara9593/caronasDistribuidas/internal/protocol"
)

func reservarItinerario(cliente *connection.Cliente, reader *bufio.Reader) {
	fmt.Print("Digite o ID do itinerário para reservar: ")
	idRide, _ := reader.ReadString('\n')
	idRide = strings.TrimSpace(idRide)

	payload := protocol.BookRideRequest{
		RideID:         idRide,
		PassengerEmail: cliente.UsuarioLogado,
	}

	resp, err := cliente.EnviarRequisicao("book_ride", payload)
	if err != nil {
		fmt.Printf("Erro de comunicação: %v\n", err)
		return
	}

	fmt.Printf("Resposta do servidor: [%s] %s\n", resp.Status, resp.Message)

	if resp.Status == "SUCCESS" && len(resp.Payload) > 0 && string(resp.Payload) != "null" {
		var resultado protocol.BookRideResult
		if err := json.Unmarshal(resp.Payload, &resultado); err == nil &&
			resultado.ReservationID != "" {
			fmt.Printf("ID da reserva: %s (guarde para cancelar depois)\n",
				resultado.ReservationID)
		}
	}
}

func cancelarReserva(cliente *connection.Cliente, reader *bufio.Reader) {
	fmt.Print("ID da reserva: ")
	reservationID, _ := reader.ReadString('\n')
	reservationID = strings.TrimSpace(reservationID)

	payload := protocol.CancelBookingRequest{
		ReservationID:  reservationID,
		PassengerEmail: cliente.UsuarioLogado,
	}

	resp, err := cliente.EnviarRequisicao("cancel_booking", payload)
	if err != nil {
		fmt.Printf("Erro de comunicação: %v\n", err)
		return
	}

	fmt.Printf("Resposta do servidor: [%s] %s\n", resp.Status, resp.Message)
}

func buscarItinerarios(cliente *connection.Cliente, reader *bufio.Reader) {
	fmt.Print("Cidade de Origem: ")
	origem, _ := reader.ReadString('\n')
	origem = strings.TrimSpace(origem)

	fmt.Print("Cidade de Destino: ")
	destino, _ := reader.ReadString('\n')
	destino = strings.TrimSpace(destino)

	fmt.Print("Data desejada (ex: 2026-09-10): ")
	data, _ := reader.ReadString('\n')
	data = strings.TrimSpace(data)

	payload := protocol.SearchRoute{
		Source:      origem,
		Destination: destino,
		ArrivalTime: data,
	}

	resp, err := cliente.EnviarRequisicao("search_route", payload)
	if err != nil {
		fmt.Printf("❌ Erro de comunicação: %v\n", err)
		return
	}

	fmt.Printf("📩 Resposta do Servidor: [%s] %s\n", resp.Status, resp.Message)

	if resp.Status != "SUCCESS" || len(resp.Payload) == 0 {
		return
	}

	var itinerarios []protocol.RideInfo
	if err := json.Unmarshal(resp.Payload, &itinerarios); err != nil {
		fmt.Printf("Erro ao interpretar a resposta: %v\n", err)
		return
	}

	if len(itinerarios) == 0 {
		fmt.Println("Nenhum itinerário encontrado para essa rota.")
		return
	}

	for i, itin := range itinerarios {
		fmt.Printf("\nItinerário %d:\n", i+1)
		fmt.Printf("  ID da rota: %s\n", itin.IdRide)
		for _, trecho := range itin.Sections {
			fmt.Printf("  Saída: %s\n", trecho.Origem)
			fmt.Printf("  Destino: %s\n", trecho.Destino)
			fmt.Printf("  Quantidade de assentos: %d\n", trecho.Assentos)
			fmt.Printf("  Empresa: %s\n", trecho.Empresa)
		}
		fmt.Printf("  Valor total da carona: R$ %.2f\n", itin.TotalPrice)
	}
}

func consultarReservas(cliente *connection.Cliente) {
	resp, err := cliente.EnviarRequisicao("get_bookings", protocol.GetBookingsRequest{
		PassengerEmail: cliente.UsuarioLogado,
	})
	if err != nil {
		fmt.Printf("Erro de comunicação: %v\n", err)
		return
	}

	fmt.Printf("Resposta do servidor: [%s] %s\n", resp.Status, resp.Message)

	if resp.Status != "SUCCESS" {
		return
	}

	var reservas []protocol.BookingInfo
	if len(resp.Payload) == 0 || string(resp.Payload) == "null" {
		fmt.Println("Você ainda não possui reservas,ou o motorista cancelou a viagem")
		return
	}

	if err := json.Unmarshal(resp.Payload, &reservas); err != nil {
		fmt.Printf("Erro ao interpretar as reservas: %v\n", err)
		return
	}

	if len(reservas) == 0 {
		fmt.Println("Você ainda não possui reservas,ou o motorista cancelou a viagem")
		return
	}

	for _, reserva := range reservas {
		fmt.Printf("\nReserva: %s\n", reserva.ReservationID)
		for _, trechoID := range reserva.SectionIDs {
			fmt.Printf("  Trecho: %s\n", trechoID)
		}
	}
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

	cliente := connection.NewCliente("Passageiro", idEmpresa, conn)
	fmt.Printf("Conectado com sucesso ao servidor da empresa %s em %s!\n", idEmpresa, endereco)

	for {
		fmt.Println("\n--- VAIJUNTO: PAINEL DO PASSAGEIRO ---")
		fmt.Println("1. Cadastrar (1º acesso)")
		fmt.Println("2. Login")
		fmt.Println("3. Buscar Itinerários de Carona")
		fmt.Println("4. Reservar Itinerário")
		fmt.Println("5. Consultar Minhas Reservas")
		fmt.Println("6. Cancelar Reserva")
		fmt.Println("7. Sair")
		fmt.Print("Escolha uma opção: ")

		opcao, _ := reader.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		precisaLogin := opcao == "3" || opcao == "4" || opcao == "5" || opcao == "6"
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
			auth.CadastrarPassageiro(cliente, reader)
		case "2":
			if cliente.IsLogged {
				fmt.Printf("Você já está logado como %s!\n", cliente.NomeLogado)
				continue
			}
			auth.LoginPassageiro(cliente, reader)
		case "3":
			buscarItinerarios(cliente, reader)
		case "4":
			reservarItinerario(cliente, reader)
		case "5":
			consultarReservas(cliente)
		case "6":
			cancelarReserva(cliente, reader)
		case "7":
			fmt.Println("Encerrando conexão...")
			return
		default:
			fmt.Println("Opção inválida.")
		}
	}
}
