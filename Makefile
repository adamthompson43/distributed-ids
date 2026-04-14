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

# AWS

AWS_KEY    := ~/.ssh/sentinel-key.pem
NODE1_IP   := 108.131.100.238
NODE2_IP   := 3.253.72.69
NODE3_IP   := 18.200.241.225
DASH_IP    := 34.244.198.157
SSH        := ssh -i $(AWS_KEY) -o StrictHostKeyChecking=no

AWS_PCAP   := s3://sentinel-pcaps-975705621736/Tuesday-WorkingHours.pcap
RDS_HOST   := sentinel-db.ctmyosyisbig.eu-west-1.rds.amazonaws.com

.PHONY: aws-run aws-stop aws-node1 aws-node2 aws-node3 aws-dash aws-logs aws-status

# start all 3 nodes
aws-run:
	$(SSH) ec2-user@$(NODE1_IP) 'pkill sentinelnode 2>/dev/null; nohup ~/run.sh > ~/node.log 2>&1 &'
	$(SSH) ec2-user@$(NODE2_IP) 'pkill sentinelnode 2>/dev/null; nohup ~/run.sh > ~/node.log 2>&1 &'
	$(SSH) ec2-user@$(NODE3_IP) 'pkill sentinelnode 2>/dev/null; nohup ~/run.sh > ~/node.log 2>&1 &'
	@echo "All 3 nodes started. Tail logs with: make aws-logs"

# stop all nodes
aws-stop:
	$(SSH) ec2-user@$(NODE1_IP) 'pkill sentinelnode 2>/dev/null; echo node1 stopped'
	$(SSH) ec2-user@$(NODE2_IP) 'pkill sentinelnode 2>/dev/null; echo node2 stopped'
	$(SSH) ec2-user@$(NODE3_IP) 'pkill sentinelnode 2>/dev/null; echo node3 stopped'

# live logs
aws-logs:
	@echo "=== node1 ===" && $(SSH) ec2-user@$(NODE1_IP) 'tail -f ~/node.log' &
	@echo "=== node2 ===" && $(SSH) ec2-user@$(NODE2_IP) 'tail -f ~/node.log' &
	@echo "=== node3 ===" && $(SSH) ec2-user@$(NODE3_IP) 'tail -f ~/node.log' &
	@wait

# ssh into each instance
aws-node1:
	$(SSH) ec2-user@$(NODE1_IP)
aws-node2:
	$(SSH) ec2-user@$(NODE2_IP)
aws-node3:
	$(SSH) ec2-user@$(NODE3_IP)
aws-dash:
	$(SSH) ec2-user@$(DASH_IP)

# status check
aws-status:
	@for ip in $(NODE1_IP) $(NODE2_IP) $(NODE3_IP); do \
	  echo -n "$$ip /healthz: "; \
	  curl -s --max-time 3 http://$$ip:8081/healthz || echo "no response"; \
	done
	@echo -n "dashboard: "; curl -s --max-time 3 http://$(DASH_IP)/api/stats | head -c 80

# pull pcap from s3 to 3 nodes
aws-pull-pcap:
	$(SSH) ec2-user@$(NODE1_IP) 'aws s3 cp $(AWS_PCAP) ~/Tuesday-WorkingHours.pcap' &
	$(SSH) ec2-user@$(NODE2_IP) 'aws s3 cp $(AWS_PCAP) ~/Tuesday-WorkingHours.pcap' &
	$(SSH) ec2-user@$(NODE3_IP) 'aws s3 cp $(AWS_PCAP) ~/Tuesday-WorkingHours.pcap' &
	@wait && echo "PCAP downloaded to all nodes"
