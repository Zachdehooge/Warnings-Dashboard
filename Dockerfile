FROM golang:1.21.0

RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN templ generate

RUN go build -o weather-warnings ./cmd

# Install Python (for simple HTTP server)
RUN apt-get update && apt-get install -y python3

# Run the Go app and then serve the generated HTML
CMD ./weather-warnings --watch -i 30 -o warnings.html -v & python3 -m http.server 8080

# To run the docker container: docker run -p 8080:8080 weather-warnings