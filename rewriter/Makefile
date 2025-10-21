all: lfi-aarch64.rst lfi-x86_64.rst

%.rst: %.rst.in
	cd table && go run main.go ../$^ > ../$@
