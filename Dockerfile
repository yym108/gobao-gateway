FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN cd /src && go build -o /out/server ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]