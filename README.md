##


 Rota 	Método 	Função
/consorcio/trechos 	GET 	Consultar trechos locais entre origem e destino
/v1/search 	GET 	Buscar itinerários no servidor
/v1/bookings 	GET 	Consultar reservas locais de um passageiro

/v1/transactions/prepare 	POST 	Preparar reserva
/v1/transactions/commit 	POST 	Confirmar reserva
/v1/transactions/abort 	    POST 	Desfazer uma preparação
/v1/transactions/prepare-cancel 	POST 	Preparar cancelamento
/v1/transactions/commit-cancel 	POST 	Confirmar cancelamento
/v1/transactions/abort-cancel 	POST 	Desfazer preparação de cancelamento


 Servidor 	IP da máquina 	TCP de exemplo 	REST de exemplo
A 	192.168.1.21 	9592 	8081
B 	192.168.1.22 	9594 	8081
C 	192.168.1.23 	9596 	8081


go run ./cmd/server -id A -tcp :9593 -http :8081 -peers "B=http://localhost:8082,C=http://localhost:8083"
 go run ./cmd/server -id B -tcp :9594 -http :8082 -peers "A=http://localhost:8081,C=http://localhost:8083"
go run ./cmd/server -id C -tcp :9595 -http :8083 -peers "A=http://localhost:8081,B=http://localhost:8082"

go run ./cmd/client/motorista

go run ./cmd/client/passageiro -empresa 