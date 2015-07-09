TAG = v0.2

build: sleepy.go 
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo --ldflags '-w' ./sleepy.go
container: 
	docker build -t saadali/sleepy:$(TAG) .
push:
	docker push saadali/sleepy:$(TAG)
clean:
	rm ./sleepy

