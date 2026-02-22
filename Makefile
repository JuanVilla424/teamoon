VERSION := 1.1.0
BINARY := teamoon
BUILD_DIR := .

.PHONY: build install clean service test sync-bmad

BUILD_DATE := $(shell date +%Y%m%d.%H%M%S)
LDFLAGS := -ldflags "-X main.buildNum=$(BUILD_DATE)"

BMAD_PKG := $(HOME)/cloud-agent-package
BMAD_SRC := $(BMAD_PKG)/.claude/commands/bmad
BMAD_MANIFEST := $(BMAD_PKG)/.bmad/_cfg/manifest.yaml
BMAD_ASSETS := internal/onboarding/assets/bmad

sync-bmad:
	@if [ -f "$(BMAD_MANIFEST)" ] && [ -d "$(BMAD_SRC)" ]; then \
		BMAD_VER=$$(python3 -c "import yaml; print(yaml.safe_load(open('$(BMAD_MANIFEST)'))['installation']['version'])" 2>/dev/null || echo "unknown"); \
		DST=$(BMAD_ASSETS)/$$BMAD_VER/commands/bmad; \
		mkdir -p $$DST; \
		cp -r $(BMAD_SRC)/* $$DST/; \
		echo "{\"latest\":\"$$BMAD_VER\",\"supported\":[\"$$BMAD_VER\"]}" > $(BMAD_ASSETS)/versions.json; \
		echo "Synced BMAD $$BMAD_VER"; \
	else \
		echo "BMAD source not found, using existing assets"; \
	fi

build: sync-bmad
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/teamoon/

install: build
	-sudo systemctl stop teamoon 2>/dev/null
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	sudo chgrp input /usr/local/bin/$(BINARY)
	sudo chmod g+s /usr/local/bin/$(BINARY)
	@sudo touch /var/log/teamoon.log 2>/dev/null || true
	@sudo chown $(shell whoami):$(shell whoami) /var/log/teamoon.log 2>/dev/null || true
	sudo cp teamoon.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable teamoon
	sudo systemctl restart teamoon

service: install

test:
	go test ./internal/...

release:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)$(SUFFIX) ./cmd/teamoon/

clean:
	rm -f $(BUILD_DIR)/$(BINARY)
