.PHONY: test build bench profile lint

test:
	CGO_ENABLED=0 go test ./...
	go test -race ./...

build:
	CGO_ENABLED=0 go build ./...

bench:
	CGO_ENABLED=0 go test . -run '^$$' -bench . -benchmem

profile:
	CGO_ENABLED=0 go test -c -o /tmp/peek-profile.test .
	/tmp/peek-profile.test -test.run '^$$' -test.bench BenchmarkReuse -test.benchtime 3s -test.cpuprofile /tmp/peek-cpu.pprof -test.memprofile /tmp/peek-mem.pprof
	go tool pprof -top /tmp/peek-profile.test /tmp/peek-cpu.pprof

lint:
	golangci-lint run --enable gocritic,gocognit,gocyclo,maintidx,dupl,mnd,unparam,ireturn,goconst,errcheck ./...
	govulncheck ./...
	deadcode -test ./...
	zizmor .github/workflows/
