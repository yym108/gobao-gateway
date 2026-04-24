FROM golang:1.22 AS build
WORKDIR /src
COPY go.work go.work
COPY gobao-pkg gobao-pkg
COPY gobao-proto gobao-proto
COPY gobao-gateway gobao-gateway
RUN cd /src/gobao-gateway && go build -o /out/server ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]