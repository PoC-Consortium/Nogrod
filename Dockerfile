FROM golang:1.10.2-stretch AS base
WORKDIR /app

FROM base AS build
COPY . .
RUN ls -la
RUN make build

FROM base AS final
COPY --from=build /app/goburstpool .
COPY --from=build /app/src/goburst/burstmath/libs/ ./src/goburst/burstmath/libs
VOLUME ["/app/config.yaml"]
EXPOSE 8124
EXPOSE 8080
EXPOSE 7777
ENTRYPOINT ./goburstpool