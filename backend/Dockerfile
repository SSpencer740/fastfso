FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/fastfso .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/fastfso /bin/fastfso
USER nobody
ENTRYPOINT ["/bin/fastfso"]
