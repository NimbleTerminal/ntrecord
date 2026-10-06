.PHONY: deps build test check install demos clean
deps:
	task deps
build:
	task build
test:
	task test
check:
	task ci
install:
	task install
demos:
	task demos
clean:
	task clean
