#!/bin/bash

set -e

REDIS_CONTAINER="my-redis"
IMITATOR_CMD="go run servers-imitator/imitator.go"
BENCH_CMD_01="go test -bench=BenchmarkBalancerWithClientSetup ./tests/benchmarks_test.go -cpuprofile=\"./tests/benchmarks-output/balancerCPU.out\" -memprofile=\"./tests/benchmarks-output/balancerMEM.out\""
BENCH_CMD_02="go test -bench=BenchmarkCreateAndDeleteClient ./tests/benchmarks_test.go -cpuprofile=\"./tests/benchmarks-output/CPU.out\" -memprofile=\"./tests/benchmarks-output/MEM.out\""
APP_CMD="go run main.go"
TESTING_CMD="go test ./tests/balancer_test.go -v"
COVERAGE_CMD="go test -coverprofile=tests/coverage-output/cover.out -coverpkg=./... ./tests/... && go tool cover -html=tests/coverage-output/cover.out -o tests/coverage-output/coverage.html"


start_redis() {
    echo "Redis is statring"
    docker run -d --name $REDIS_CONTAINER -p 6379:6379 redis
    sleep 2
}

stop_redis() {
    echo "Stopping Redis"
    docker stop $REDIS_CONTAINER >/dev/null 2>&1 || true
    docker rm $REDIS_CONTAINER >/dev/null 2>&1 || true
}

start_imitator() {
    echo "Starting server-imitator"
    eval $IMITATOR_CMD
}

run_benchmarks() {
    echo "Starting benchmarks. Their result will be located at tests/benchmarks-output"
    eval $BENCH_CMD_01
    eval $BENCH_CMD_02
}

run_app() {
    echo "Starting main program"
    eval $APP_CMD
}

run_tests() {
    echo "Starting testing"
    eval $TESTING_CMD
}

run_coverage() {
    echo "Starting coverage. It'll be located it tests/coverage-output/coverage.html"
    eval $COVERAGE_CMD
}


case "$1" in
    "redis")
        start_redis
        ;;
    "stop-redis")
        stop_redis
        ;;
    "imitator")
        start_imitator
        ;;
    "bench")
        run_benchmarks
        ;;
    "main")
        run_app
        ;;
    "test")
        run_tests
        ;;
    "coverage")
        run_coverage
        ;;
    *)
        echo "Доступные команды:"
        echo "  redis - Запустить только Redis"
        echo "  stop-redis - Остановить Redis"
        echo "  imitator - Запустить server-imitator"
        echo "  main - Запустить саму программу"
        echo "  bench - Запустить бенчмарки"
        echo "  test - Запустить тесты"
        echo "  coverage - Запустить тест покрытия"
        exit 1
        ;;
esac