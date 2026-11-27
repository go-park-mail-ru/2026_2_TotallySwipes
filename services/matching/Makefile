SERVICE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
ROOT_DIR := $(abspath $(SERVICE_DIR)/../..)
PROTO_FILES := $(shell find "$(SERVICE_DIR)/api/proto" -type f -name '*.proto' | sort)

.PHONY: proto
proto:
	protoc \
		--proto_path="$(SERVICE_DIR)/api/proto" \
		--proto_path="$(ROOT_DIR)/api/proto" \
		--go_out="$(ROOT_DIR)" \
		--go_opt=module=dating-app \
		--go-grpc_out="$(ROOT_DIR)" \
		--go-grpc_opt=module=dating-app \
		$(PROTO_FILES)
