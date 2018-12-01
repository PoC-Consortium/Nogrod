FROM golang:1.11.2-stretch AS base
WORKDIR /app

FROM base AS build
COPY . .
RUN make build-docker

FROM base AS final
COPY --from=build /app/Nogrod .
COPY --from=build /app/migrations/ ./migrations
COPY --from=build /app/src/goburst/burstmath/libs/ ./src/goburst/burstmath/libs
COPY --from=build /app/web ./web
VOLUME ["/app/config.yaml"]
EXPOSE 8124
EXPOSE 8080
EXPOSE 7777
ENTRYPOINT ./Nogrod