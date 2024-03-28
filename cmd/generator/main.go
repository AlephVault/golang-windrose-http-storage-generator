package main

import (
	"flag"
	"fmt"
	"github.com/AlephVault/golang-windrose-http-storage-generator/cmd/generator/templates"
	"os"
	"path/filepath"
	"strings"
)

var dockerComposeFileContentsTemplate = strings.TrimSpace(`
version: '3.7'
services:
  express:
    image: mongo-express:1.0.0-alpha
    restart: always
    env_file: .env
    ports:
      - %d:8081
    expose:
      - %d
  mongodb:
    image: mongo:6.0
    restart: always
    env_file: .env
    ports:
      - %d:27017
    expose:
      - %d
    volumes:
      - .tmp/mongo:/data/db
  http:
    build:
      context: ./server
    command: waitress-serve --listen=0.0.0.0:80 app:app
    restart: always
    env_file: .env
    ports:
      - %d:80
    expose:
      - %d
`)

var dockerComposeLauncherFileContents = strings.TrimSpace(`
#!/bin/bash
DIR="$(dirname "$0")"
(cd "$DIR" && docker-compose $@)
`)

var envFileContentsTemplate = strings.TrimSpace(`
# These environment variables stand for all the containers
MONGO_INITDB_ROOT_USERNAME=%s
MONGO_INITDB_ROOT_PASSWORD=%s
DB_HOST=mongodb
DB_PORT=27017
DB_USER=%s
DB_PASS=%s
ME_CONFIG_MONGODB_SERVER=mongodb
ME_CONFIG_MONGODB_PORT=27017
ME_CONFIG_MONGODB_ADMINUSERNAME=%s
ME_CONFIG_MONGODB_ADMINPASSWORD=%s
SERVER_API_KEY=%s
`)

var moduleFileContents = strings.TrimSpace(`
module my-project

// You might want to change the golang version.
go 1.22

require github.com/AlephVault/golang-standard-http-mongodb-storage v1.1.1
`)

var dockerFileContents = strings.TrimSpace(`
FROM golang:1.22 AS builder
WORKDIR /app
COPY ./ /app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o myapp ./main.go

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/myapp .
CMD ["./myapp"]
`)

// dumpFile dumps a file's contents.
func dumpFile(filePath, content string, perm os.FileMode) {
	if err := os.WriteFile(filePath, []byte(content), perm); err != nil {
		panic("could not dump file " + filePath + ": " + err.Error())
	}
}

// makeDockerComposeFile makes and dumps the contents of the compose file.
func makeDockerComposeFile(projectPath string, mongoPort uint16, httpPort uint16, mongoExpressPort uint16) {
	// Suggested ports: mongo=27017, http=8080, express=8081.
	dumpFile(filepath.Join(projectPath, "docker-compose.yml"), fmt.Sprintf(
		dockerComposeFileContentsTemplate,
		mongoExpressPort, mongoExpressPort,
		mongoPort, mongoPort,
		httpPort, httpPort,
	), 0644)
}

// makeDockerComposeLauncherFile makes and dumps the contents of the script that launches the compose file.
func makeDockerComposeLauncherFile(projectPath string) {
	dumpFile(filepath.Join(projectPath, "compose.sh"), dockerComposeLauncherFileContents, 0755)
}

// makeEnvFile makes the suitable env file.
func makeEnvFile(projectPath, mongoUser, mongoPass, serverAPIKey string) {
	dumpFile(filepath.Join(projectPath, ".env"), fmt.Sprintf(
		envFileContentsTemplate,
		mongoUser, mongoPass,
		mongoUser, mongoPass,
		mongoUser, mongoPass,
		serverAPIKey,
	), 0644)
}

// makeModuleFile creates the go.mod file.
func makeModuleFile(projectPath string) {
	dumpFile(filepath.Join(projectPath, "server", "go.mod"), moduleFileContents, 0644)
}

// makeDockerFile creates the proper dockerfile contents.
func makeDockerFile(projectPath string) {
	dumpFile(filepath.Join(projectPath, "server", "Dockerfile"), dockerFileContents, 0644)
}

// makeAppFile creates the contents of the app file depending on the chosen template.
func makeAppFile(projectPath, template string) {
	contents := ""
	if template == "default:simple" {
		contents = templates.SimpleAppTemplate
	} else if template == "default:multichar" {
		contents = templates.MultipleAppTemplates
	} else {
		if content, err := os.ReadFile(template); err == nil {
			contents = string(content)
		} else {
			panic("could not read template file " + template + ": " + err.Error())
		}
	}

	dumpFile(filepath.Join(projectPath, "server", "main.go"), contents, 0644)
}

// generateProject generates an entire project stack.
// This one will be only suitable for development.
func generateProject(
	projectPath, template string,
	mongoPort, httpPort, mongoExpressPort uint16,
	mongoUser, mongoPass, serverAPIKey string,
) {
	if err := os.MkdirAll(filepath.Join(projectPath, "server"), 0755); err != nil {
		panic("could not create project directory " + projectPath + ": " + err.Error())
	}
	makeDockerComposeFile(projectPath, mongoPort, httpPort, mongoExpressPort)
	makeDockerComposeLauncherFile(projectPath)
	makeEnvFile(projectPath, mongoUser, mongoPass, serverAPIKey)
	makeDockerFile(projectPath)
	makeModuleFile(projectPath)
	makeAppFile(projectPath, template)
}

func main() {
	defer func() {
		if v := recover(); v != nil {
			_, _ = fmt.Fprintln(os.Stderr, "Error on generation:", v)
		}
	}()

	// Define flags
	projectPath := flag.String("projectPath", "", "Path to the project (mandatory)")
	template := flag.String("template", "", "Template to use (\"default:simple\", \"default:multichar\" or a path to a file)")
	mongoDBPort := flag.Uint("mongoDBPort", 27017, "MongoDB port to use")
	httpPort := flag.Uint("httpPort", 8080, "HTTP port to use")
	mongoDBExpressPort := flag.Uint("mongoDBExpressPort", 8081, "MongoDB Express port to use")
	mongoDBUser := flag.String("mongoDBUser", "admin", "MongoDB user")
	mongoDBPassword := flag.String("mongoDBPassword", "p455w0rd", "MongoDB password")
	defaultAPIKey := flag.String("defaultAPIKey", "sample-abcdef", "Default server API key")

	// Parse the flags
	flag.Parse()

	// Check for mandatory string arguments
	if *projectPath == "" || *template == "" {
		_, _ = fmt.Fprintln(os.Stderr, "projectPath and template are required.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	generateProject(
		*projectPath, *template,
		uint16(*mongoDBPort), uint16(*httpPort), uint16(*mongoDBExpressPort),
		*mongoDBUser, *mongoDBPassword, *defaultAPIKey,
	)
}
