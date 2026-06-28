FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /go/src
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -installsuffix cgo -o app .

FROM gcr.io/distroless/static
COPY --from=builder /go/src/app .
CMD ["/app"]