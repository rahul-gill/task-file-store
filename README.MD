Below operations can be run using the makefile

#  Building
Run these in root of project
- `go mod download`
- build client   
 `GOOS=linux GOARCH=amd64 go build -o store ./client/store_client.go`
- build server  
  `GOOS=linux GOARCH=amd64 go build -o store_server ./server/store_server.go`

# Using with docker
- ensure docker and docker-buildx are installed
- run this in root of project to build docker image  
  `docker build -t file_store_server .`
- Run the container and expose on 8080 port  
  `docker run  -p 8080:8080 file_store_server:latest`

# Deploying on kubernetes using local minikube cluster
- Ensure docker service is running
- Run minikube using `minkube start`
- set the env `eval $(minikube docker-env)`
- Now `docker build -t my-file_store_server .` will build the image using minikube docker daemon
- Now apply the deployment files
  - `kubectl apply -f ./config/file-store-server-deployment.yaml`
  - `kubectl apply -f ./config/file-store-server-pv-pvc.yaml`
  - `kubectl apply -f ./config/file-store-server-service.yaml`
- Forward file store service ports for local use
  - `kubectl port-forward svc/file-store-server-service 8080:8080`
- Now you can use the previously built `store` client binary to use the file store service
