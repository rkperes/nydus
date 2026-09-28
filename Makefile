# Local knobs. Copy .env.example to .env and edit. Anything set in .env wins;
# the ?= defaults below apply only when a value is unset.
-include .env

SERVER            ?= pc
CLUSTER           := lab
REG_PORT          := 5000
# Address baked into the API server's TLS SANs. Inferred from the server at
# cluster-up time — its tailnet IP if tailscale is present, else its primary IP.
# Set it explicitly when the inference is wrong.
API_SERVER_ADDRESS ?=
KUBECONFIG_FILE   ?= $(HOME)/.kube/nydus-$(CLUSTER).yaml

.PHONY: bootstrap cluster-up cluster-down kubeconfig tunnel tunnel-stop status nuke

bootstrap:
	ssh $(SERVER) 'bash -s' < deploy/bootstrap-server.sh

cluster-up:
	@if [ -n "$(API_SERVER_ADDRESS)" ]; then \
	  ip="$(API_SERVER_ADDRESS)"; \
	else \
	  ip=$$(ssh $(SERVER) 'tailscale ip -4 2>/dev/null | head -1'); \
	  [ -n "$$ip" ] || ip=$$(ssh $(SERVER) 'hostname -I 2>/dev/null | cut -d" " -f1'); \
	fi; \
	[ -n "$$ip" ] || { echo "could not infer the server address from $(SERVER); set API_SERVER_ADDRESS in .env" >&2; exit 1; }; \
	sed "s/__API_SERVER_ADDRESS__/$$ip/" kind.yaml | ssh $(SERVER) 'cat > /tmp/nydus-kind.yaml'
	ssh $(SERVER) 'bash -s' < deploy/cluster-up.sh
	$(MAKE) kubeconfig

cluster-down:
	ssh $(SERVER) 'kind delete cluster --name $(CLUSTER)'

kubeconfig:
	@mkdir -p $(dir $(KUBECONFIG_FILE))
	ssh $(SERVER) 'kind get kubeconfig --name $(CLUSTER)' > $(KUBECONFIG_FILE)
	@chmod 600 $(KUBECONFIG_FILE)
	@echo "run: export KUBECONFIG=$(KUBECONFIG_FILE)"

tunnel:
	@pgrep -f "ssh.*-L $(REG_PORT):127.0.0.1:$(REG_PORT)" >/dev/null \
		|| ssh -fN -L $(REG_PORT):127.0.0.1:$(REG_PORT) $(SERVER)
	@echo "registry tunnel up on localhost:$(REG_PORT)"

tunnel-stop:
	@pkill -f "ssh.*-L $(REG_PORT):127.0.0.1:$(REG_PORT)" || true

status:
	kubectl --kubeconfig $(KUBECONFIG_FILE) get nodes -o wide
	kubectl --kubeconfig $(KUBECONFIG_FILE) get pods -A

nuke: cluster-down tunnel-stop
	ssh $(SERVER) 'docker system prune -f'
