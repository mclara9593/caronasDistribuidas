package graph

import (
	"testing"
)

func TestAddRideAssentosPorTrecho(t *testing.T) {
	g := NovoGrafo()
	rota := []string{"Feira", "Salvador", "Aracaju"}
	assentos := []int{3, 1}
	precos := []float64{25.0, 50.0}

	err := g.AddRide("ride-1", rota, assentos, precos, "motorista@teste.com")
	if err != nil {
		t.Fatalf("erro inesperado ao adicionar carona: %v", err)
	}

	ride, existe := g.Rides["ride-1"]
	if !existe {
		t.Fatalf("carona não encontrada no grafo")
	}

	if len(ride.Sections) != 2 {
		t.Fatalf("esperava 2 seções, obteve %d", len(ride.Sections))
	}

	if ride.Sections[0].Assentos != 3 {
		t.Errorf("esperava 3 assentos no primeiro trecho, obteve %d", ride.Sections[0].Assentos)
	}
	if ride.Sections[1].Assentos != 1 {
		t.Errorf("esperava 1 assento no segundo trecho, obteve %d", ride.Sections[1].Assentos)
	}

	// Testa validação com tamanho incompatível
	errInvalido := g.AddRide("ride-2", rota, []int{3}, precos, "motorista@teste.com")
	if errInvalido == nil {
		t.Errorf("esperava erro por tamanho incompatível de assentos")
	}

	// Testa assento <= 0
	errZero := g.AddRide("ride-3", rota, []int{3, 0}, precos, "motorista@teste.com")
	if errZero == nil {
		t.Errorf("esperava erro por assento menor ou igual a zero")
	}
}

