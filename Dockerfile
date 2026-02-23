FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/dbapi ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/dbapi /usr/local/bin/dbapi

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/dbapi"]
