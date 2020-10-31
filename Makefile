docker-build:
	docker build -t room-crawler .

docker-push:
	docker tag room-crawler gcr.io/room-crawler/room-crawler
	docker push gcr.io/room-crawler/room-crawler
