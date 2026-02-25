VERSION := 1.1.3
BINARY := teamoon
BUILD_DIR := .

.PHONY: build install clean service test sync-bmad

BUILD_DATE := $(shell date +%Y%m%d.%H%M%S)
LDFLAGS := -ldflags "-X main.buildNum=$(BUILD_DATE)"

BMAD_PKG := $(HOME)/cloud-agent-package
BMAD_SRC := $(BMAD_PKG)/.claude/commands/bmad
BMAD_MANIFEST := $(BMAD_PKG)/.bmad/_cfg/manifest.yaml
BMAD_ASSETS := internal/onboarding/assets/bmad

DISTRO_FAMILY := $(shell \
	if [ -f /etc/debian_version ]; then echo debian; \
	elif [ -f /etc/redhat-release ] || [ -f /etc/rocky-release ]; then echo rhel; \
	else echo unknown; fi)

sync-bmad:
	@if [ -f "$(BMAD_MANIFEST)" ] && [ -d "$(BMAD_SRC)" ]; then \
		BMAD_VER=$$(python3 -c "import yaml; print(yaml.safe_load(open('$(BMAD_MANIFEST)'))['installation']['version'])" 2>/dev/null || echo "unknown"); \
		DST=$(BMAD_ASSETS)/$$BMAD_VER/commands/bmad; \
		mkdir -p $$DST; \
		cp -r $(BMAD_SRC)/* $$DST/; \
		echo "{\"latest\":\"$$BMAD_VER\",\"supported\":[\"$$BMAD_VER\"]}" > $(BMAD_ASSETS)/versions.json; \
		echo "Synced BMAD $$BMAD_VER"; \
	fi

build: sync-bmad
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/teamoon/

install: build check-deps
	-sudo systemctl stop teamoon 2>/dev/null
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	sudo chmod 755 /usr/local/bin/$(BINARY)
	@sudo touch /var/log/teamoon.log 2>/dev/null || true
	@sudo chown $(shell whoami):$(shell whoami) /var/log/teamoon.log 2>/dev/null || true
	@CURRENT_USER=$$(whoami); \
	HOME_DIR=$$(eval echo ~$$CURRENT_USER); \
	if [ "$(DISTRO_FAMILY)" = "rhel" ]; then \
		ENV_PATH="/etc/sysconfig/teamoon"; \
	else \
		ENV_PATH="$$HOME_DIR/.config/teamoon/.env"; \
	fi; \
	printf '[Unit]\nDescription=Teamoon - AI-powered project management and autopilot task engine\nAfter=network.target\n\n[Service]\nType=simple\nUser=%s\nGroup=%s\nExecStart=/usr/local/bin/teamoon serve\nRestart=always\nRestartSec=5\nWorkingDirectory=%s\nEnvironment=HOME=%s\nEnvironmentFile=-%s\n\n[Install]\nWantedBy=multi-user.target\n' \
		"$$CURRENT_USER" "$$CURRENT_USER" "$$HOME_DIR" "$$HOME_DIR" "$$ENV_PATH" \
		> teamoon.service
	@if [ "$(DISTRO_FAMILY)" = "rhel" ]; then \
		if command -v restorecon >/dev/null 2>&1; then \
			sudo restorecon -v /usr/local/bin/$(BINARY) 2>/dev/null || true; \
			sudo restorecon -v /var/log/teamoon.log 2>/dev/null || true; \
		fi; \
		if [ ! -f /etc/sysconfig/teamoon ]; then \
			printf '# teamoon environment\n' | sudo tee /etc/sysconfig/teamoon >/dev/null; \
			sudo chmod 640 /etc/sysconfig/teamoon; \
		fi; \
	fi
	sudo cp teamoon.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable teamoon
	sudo systemctl restart teamoon

check-deps:
	@command -v expect >/dev/null 2>&1 || { \
		echo "Installing expect (required for claude /usage)..."; \
		if [ "$(DISTRO_FAMILY)" = "rhel" ]; then \
			sudo dnf install -y expect 2>/dev/null || sudo yum install -y expect; \
		else \
			sudo apt-get install -y expect; \
		fi; \
	}

service: install

test:
	go test ./internal/...

release:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)$(SUFFIX) ./cmd/teamoon/

clean:
	rm -f $(BUILD_DIR)/$(BINARY)
