all: test

test_fmt:
	@echo Checking correct formatting of files
	@{ \
		files=$$( go fmt ./... ); \
		if [ -n "$$files" ]; then \
		echo "Files not properly formatted: $$files"; \
		exit 1; \
		fi; \
		if ! go vet ./...; then \
		exit 1; \
		fi \
	}

test_lint:
	@echo Checking linting of files
	@{ \
		go get -u github.com/golang/lint/golint; \
		lintfiles=$$( golint ./... ); \
		if [ -n "$$lintfiles" ]; then \
		echo "Lint errors:"; \
		echo "$$lintfiles"; \
		exit 1; \
		fi \
	}

test_verbose:
	go test -v -race -short ./...

# use test_verbose instead if you want to use this Makefile locally
test_go:
	go test -race -short ./...

test: test_fmt test_lint test_go