build: simmemleak.go 
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo --ldflags '-w' ./simmemleak.go
container: 
	docker build -t saadali/simmemleak .
push:
	docker push saadali/simmemleak
clean:
	rm ./simmemleak

