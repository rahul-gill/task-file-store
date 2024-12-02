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