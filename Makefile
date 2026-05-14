BIN := talos-fake-apid
TARGET ?= 192.168.0.23
TARGET_USER ?= ubuntu
TARGET_DIR ?= /home/ubuntu/talos-fake-apid

.PHONY: build
build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o $(BIN) .

.PHONY: build-local
build-local:
	go build -o $(BIN) .

.PHONY: fetch-ca
fetch-ca:
	talosctl -n 192.168.0.10 get osrootsecret os -o yaml > /tmp/osrootsecret.yaml
	python3 -c "import yaml, base64; d=list(yaml.safe_load_all(open('/tmp/osrootsecret.yaml')))[0]['spec']; open('ca.crt','wb').write(base64.b64decode(d['issuingCA']['crt'])); open('ca.key','wb').write(base64.b64decode(d['issuingCA']['key']))"

.PHONY: fetch-config
fetch-config:
	talosctl -n 192.168.0.20 get mc v1alpha1 -o yaml | python3 -c "import yaml, sys; print(list(yaml.safe_load_all(sys.stdin))[0]['spec'])" > machine-config.yaml

.PHONY: deploy
deploy: build
	ssh $(TARGET_USER)@$(TARGET) "mkdir -p $(TARGET_DIR)"
	scp $(BIN) ca.crt ca.key machine-config.yaml $(TARGET_USER)@$(TARGET):$(TARGET_DIR)/

.PHONY: run-remote
run-remote:
	ssh $(TARGET_USER)@$(TARGET) "cd $(TARGET_DIR) && sudo pkill -f $(BIN) || true; sudo nohup ./$(BIN) > server.log 2>&1 & sleep 1; echo started"

.PHONY: stop-remote
stop-remote:
	ssh $(TARGET_USER)@$(TARGET) "sudo pkill -f $(BIN) || true"

.PHONY: logs
logs:
	ssh $(TARGET_USER)@$(TARGET) "tail -n 200 $(TARGET_DIR)/server.log"
