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

# Máquina A (substitua IPB e IPC pelos IPs reais das máquinas B e C)
go run ./cmd/server -id A -tcp :9592 -http :8081 \
  -peers "B=http://IPB:8081,C=http://IPC:8081"

# Máquina B (substitua IPA e IPC pelos IPs reais das máquinas A e C)
go run ./cmd/server -id B -tcp :9594 -http :8081 \
  -peers "A=http://IPA:8081,C=http://IPC:8081"

# Máquina C (substitua IPA e IPB pelos IPs reais das máquinas A e B)
go run ./cmd/server -id C -tcp :9596 -http :8081 \
  -peers "A=http://IPA:8081,B=http://IPB:8081"

# Conectar clientes remotamente (passando a empresa e o IP:porta da máquina desejada):
go run ./cmd/client/motorista -empresa A -addr 192.168.1.21:9592
go run ./cmd/client/passageiro -empresa B -addr 192.168.1.22:9594 