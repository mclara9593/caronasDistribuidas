package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/mclara9593/caronasDistribuidas/internal/server"
)

func main() {
	id := flag.String("id", "", "ID desta empresa: A, B ou C")
	tcpAddr := flag.String("tcp", "", "Endereço TCP para clientes, por exemplo :9593")
	httpAddr := flag.String("http", "", "Endereço REST para os outros servidores, por exemplo :8081")
	peersArg := flag.String(
		"peers",
		"",
		"Servidores REST remotos no formato ID=URL, separados por vírgula; por exemplo B=http://localhost:8082,C=http://localhost:8083",
	)
	flag.Parse()

	if *id == "" || *tcpAddr == "" || *httpAddr == "" {
		log.Fatal("informe -id, -tcp e -http")
	}

	peers, err := parsePeers(*peersArg)
	if err != nil {
		log.Fatalf("configuração de peers inválida: %v", err)
	}

	if _, existe := peers[*id]; existe {
		log.Fatalf("o servidor não deve listar a si mesmo em -peers")
	}

	cfg := server.Config{
		IDServidor: *id,
		TCPAddr:    *tcpAddr,
		HTTPAddr:   *httpAddr,
		Peers:      peers,
	}

	srv := server.NovoServidor(cfg)

	log.Printf(
		"Iniciando empresa %s (TCP %s, REST %s), peers: %v",
		*id,
		*tcpAddr,
		*httpAddr,
		peers,
	)

	if err := srv.Iniciar(); err != nil {
		log.Fatalf("erro ao iniciar servidor %s: %v", *id, err)
	}
}

func parsePeers(valor string) (map[string]string, error) {
	peers := make(map[string]string)

	if strings.TrimSpace(valor) == "" {
		return peers, nil
	}

	for _, entrada := range strings.Split(valor, ",") {
		partes := strings.SplitN(strings.TrimSpace(entrada), "=", 2)
		if len(partes) != 2 {
			return nil, fmt.Errorf(
				"entrada %q: esperado ID=http://host:porta",
				entrada,
			)
		}

		id := strings.TrimSpace(partes[0])
		url := strings.TrimRight(strings.TrimSpace(partes[1]), "/")

		if id == "" || url == "" {
			return nil, fmt.Errorf("entrada incompleta: %q", entrada)
		}

		if _, repetido := peers[id]; repetido {
			return nil, fmt.Errorf("ID de servidor repetido: %s", id)
		}

		peers[id] = url
	}

	return peers, nil
}

//go run ./cmd/server -id A -tcp :9593 -http :8081 -peers "B=http://localhost:8082,C=http://localhost:8083"
//go run ./cmd/server -id B -tcp :9594 -http :8082 -peers "A=http://localhost:8081,C=http://localhost:8083"
//go run ./cmd/server -id C -tcp :9595 -http :8083 -peers "A=http://localhost:8081,B=http://localhost:8082"
