# for macos
CGO_CFLAGS  ?= -I/opt/homebrew/include
CGO_LDFLAGS ?= -L/opt/homebrew/lib

.PHONY: build run clean

build:
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	go build -o sentinelnode .

run: build
	sudo ./sentinelnode --interface en0 --model ../model_params.json

# Local 3-node test: run each target in a separate terminal.
# Optionally pass a PostgreSQL DSN:
#   make node1 DB=postgres://ids:pass@localhost/distributed_ids?sslmode=disable
PCAP ?= /Volumes/T7/PCAPS/pcap/Tuesday-WorkingHours.pcap
DB   ?=

# Append -db flag only when DB is non-empty
DB_FLAG = $(if $(DB),-db=$(DB))

node1: build
	./sentinelnode -pcap "$(PCAP)" -model ./model_node1.json \
		-node-id node1 -listen :8081 -peers http://localhost:8082,http://localhost:8083 \
		$(DB_FLAG)

node2: build
	./sentinelnode -pcap "$(PCAP)" -model ./model_node2.json \
		-node-id node2 -listen :8082 -peers http://localhost:8081,http://localhost:8083 \
		$(DB_FLAG)

node3: build
	./sentinelnode -pcap "$(PCAP)" -model ./model_node3.json \
		-node-id node3 -listen :8083 -peers http://localhost:8081,http://localhost:8082 \
		$(DB_FLAG)

clean:
	rm -f sentinelnode
