TAG = v0.2

build: sleepy.c 
	gcc -static -O3 ./sleepy.c -o sleepy
container: 
	docker build -t saadali/sleepy:$(TAG) .
push:
	docker push saadali/sleepy:$(TAG)
clean:
	rm ./sleepy
