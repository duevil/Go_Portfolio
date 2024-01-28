FROM golang:1.21.3 AS build

WORKDIR /portfolio
COPY ./portfolio .
RUN mkdir -p vendor
COPY go.mod .
COPY go.sum .
RUN go mod vendor
RUN go build -o Portfolio .

FROM debian:stable-20231120-slim
WORKDIR /app
COPY --from=build /portfolio/Portfolio .
RUN mkdir -p /data/files
RUN mkdir -p /data/templates
COPY files* data/files/
COPY templates* data/templates/
EXPOSE 9000
CMD ["/app/Portfolio"]
