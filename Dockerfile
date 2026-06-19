FROM yatori-go-desktop:0.3.4

LABEL org.opencontainers.image.title="yatori-go-desktop"
LABEL org.opencontainers.image.version="0.3.5"

WORKDIR /src/yatori-go-desktop

COPY . .

ENV CGO_ENABLED=1

CMD ["bash", "-lc", "go test ./service/... -v"]

