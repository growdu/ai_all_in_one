# --- build ---
.PHONY: build
build:
	go build -o bin/aiio ./cmd/aiio

# --- run ---
.PHONY: run-master
run-master: build
	AIIO_ROLE=master AIIO_CONFIG=configs/master.yaml ./bin/aiio

.PHONY: run-worker-cn
run-worker-cn: build
	AIIO_ROLE=worker AIIO_REGION=cn AIIO_CONFIG=configs/worker-cn.yaml ./bin/aiio

.PHONY: run-worker-us
run-worker-us: build
	AIIO_ROLE=worker AIIO_REGION=us-west AIIO_CONFIG=configs/worker-us.yaml ./bin/aiio

# --- test ---
.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

# --- 容器 ---
.PHONY: docker-build
docker-build:
	docker build -t aiio:latest .
