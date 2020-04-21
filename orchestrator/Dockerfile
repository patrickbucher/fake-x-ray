FROM golang:1.14.2-buster AS builder
LABEL maintainer="Patrick Bucher <patrick.bucher@stud.hslu.ch>"
WORKDIR /src
COPY . .
RUN go mod tidy
RUN go build -o /app/orchestrator

FROM debian:buster-slim
LABEL maintainer="Patrick Bucher <patrick.bucher@stud.hslu.ch>"
COPY --from=builder /app/orchestrator /bin/orchestrator
EXPOSE 8080
RUN groupadd -g 1001 gophers && useradd -g 1001 -u 1001 -M gopher
CMD ["/bin/orchestrator"]
