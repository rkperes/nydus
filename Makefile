SERVER          ?= pc
CLUSTER         := lab
REG_PORT        := 5000
KUBECONFIG_FILE := $(HOME)/.kube/nydus-$(CLUSTER).yaml

.PHONY: bootstrap cluster-up cluster-down kubeconfig tunnel tunnel-stop status nuke

bootstrap:
	ssh $(SERVER) 'bash -s' < deploy/bootstrap-server.sh

cluster-up:
	cat kind.yaml | ssh $(SERVER) 'cat > /tmp/nydus-kind.yaml'
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
