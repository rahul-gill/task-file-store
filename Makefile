build-client:
	GOOS=linux GOARCH=amd64 go build -o store ./client/store_client.go
build-server:
	GOOS=linux GOARCH=amd64 go build -o store_server ./server/store_server.go
build-docker-image:
	docker build -t file_store_server .
run-server-on-docker:
	docker build -t file_store_server . && docker run  -p 8080:8080 file_store_server:latest
start-minikube:
	minkube start
minkube-dashboard:
	minikube dashboard
deploy-on-k8s:
	eval $(minikube docker-env)
	docker build -t my-file_store_server .
	kubectl apply -f ./config/file-store-server-deployment.yaml
	kubectl apply -f ./config/file-store-server-pv-pvc.yaml
	kubectl apply -f ./config/file-store-server-service.yaml
	kubectl port-forward svc/file-store-server-service 8080:8080

.PHONY: build-client build-server build-docker-image run-server-on-docker start-minikube minkube-dashboard \
	deploy-on-k8s