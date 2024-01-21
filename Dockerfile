FROM golang:1.21.3 AS build

WORKDIR /portfolio
COPY ./internal ./internal
COPY ./portfolio .
RUN mkdir -p vendor
COPY go.mod .
COPY go.sum .
RUN go mod vendor
RUN go build -o Portfolio .

FROM debian:stable-20231120-slim
WORKDIR /app
COPY --from=build /portfolio/Portfolio .
COPY static/ data/static/
COPY templates/ data/templates/
EXPOSE 9000
CMD ["/app/Portfolio"]
