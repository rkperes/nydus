include VERSIONS

NS := nydus

.PHONY: up down status logs ns port-forward

up:
	sed -e 's/__TEMPORAL_SERVER_VERSION__/$(TEMPORAL_SERVER)/g' \
	    -e 's/__TEMPORAL_UI_VERSION__/$(TEMPORAL_UI)/g' \
	    -e 's/__TEMPORAL_ADMIN_TOOLS_VERSION__/$(TEMPORAL_ADMIN_TOOLS)/g' \
	    -e 's/__POSTGRES_VERSION__/$(POSTGRES)/g' \
	    manifest.yaml | kubectl apply -f -
	kubectl -n $(NS) rollout status deployment/temporal-postgres --timeout=300s
	kubectl -n $(NS) rollout status deployment/temporal --timeout=300s
	kubectl -n $(NS) rollout status deployment/temporal-ui --timeout=300s

down:
	kubectl -n $(NS) delete deploy,svc,pvc,secret \
	  -l app.kubernetes.io/part-of=temporal --ignore-not-found

status:
	kubectl -n $(NS) get pods,svc -l app.kubernetes.io/part-of=temporal

logs:
	kubectl -n $(NS) logs -f deployment/temporal

ns:
	kubectl -n $(NS) exec deployment/temporal -- \
	  temporal --address temporal:7233 operator namespace create --namespace nydus

port-forward:
	kubectl -n $(NS) port-forward svc/temporal-ui 8080:8080
