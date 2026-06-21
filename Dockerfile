FROM yatori-go-desktop:0.3.4

LABEL org.opencontainers.image.title="yatori-go-desktop"
LABEL org.opencontainers.image.version="0.3.7"

WORKDIR /src/yatori-go-desktop

COPY . .

ENV CGO_ENABLED=1
ENV PATH="/usr/local/go/bin:${PATH}"

CMD ["bash", "-lc", "/usr/local/go/bin/go test ./service/... -v"]

