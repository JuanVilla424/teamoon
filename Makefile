VERSION := 1.1.0
BINARY := teamoon
BUILD_DIR := .

.PHONY: build install clean service test

BUILD_DATE := $(shell date +%Y%m%d.%H%M%S)
LDFLAGS := -ldflags "-X main.buildNum=$(BUILD_DATE)"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/teamoon/

install: build
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	sudo chgrp input /usr/local/bin/$(BINARY)
	sudo chmod g+s /usr/local/bin/$(BINARY)
	@sudo touch /var/log/teamoon.log 2>/dev/null || true
	@sudo chown $(shell whoami):$(shell whoami) /var/log/teamoon.log 2>/dev/null || true

service: build
	-sudo systemctl stop teamoon 2>/dev/null
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	sudo chgrp input /usr/local/bin/$(BINARY)
	sudo chmod g+s /usr/local/bin/$(BINARY)
	sudo cp teamoon.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable teamoon
	sudo systemctl restart teamoon

test:
	go test ./internal/...

clean:
	rm -f $(BUILD_DIR)/$(BINARY)
