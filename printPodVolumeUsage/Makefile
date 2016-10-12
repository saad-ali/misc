build: printPodVolumeUsage.go 
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo --ldflags '-w' ./printPodVolumeUsage.go
container: 
	docker build -t saadali/printpodvolumeusage .
push:
	docker push saadali/printpodvolumeusage
clean:
	rm ./printPodVolumeUsage
