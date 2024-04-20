VERSION=v0.1.2

build:
	docker build -t fako1024/cmdchat-server:$(VERSION) -f Dockerfile.server .

push:
	docker push fako1024/cmdchat-server:$(VERSION)

release:
	git tag $(VERSION)
	git push origin $(VERSION)
