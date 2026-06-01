FROM golang:1.26-alpine AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
COPY data/ /src/data/

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

FROM alpine:3.22

WORKDIR /app/backend

COPY --from=build /out/api /usr/local/bin/backtestsim-api
COPY --from=build /out/worker /usr/local/bin/backtestsim-worker
COPY --from=build /src/data /app/data

EXPOSE 8080

CMD ["backtestsim-api"]