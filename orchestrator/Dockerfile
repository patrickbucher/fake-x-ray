FROM golang:1.14.2-buster AS builder
LABEL maintainer="Patrick Bucher <patrick.bucher@stud.hslu.ch>"
RUN apt-get update && apt-get install -y git ca-certificates
COPY cmd/main.go go.mod go.sum /src/
WORKDIR /src
RUN go mod tidy
RUN go build -o /app/orchestrator

FROM debian:buster-slim
LABEL maintainer="Patrick Bucher <patrick.bucher@stud.hslu.ch>"
COPY --from=builder /app/orchestrator /bin/orchestrator
ENV HTTP_PORT="8080"
EXPOSE $HTTP_PORT
RUN groupadd -g 1001 gophers && useradd -g 1001 -u 1001 -M gopher
CMD ["/bin/orchestrator"]
