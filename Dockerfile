FROM gcr.io/distroless/static-debian12
COPY fin /usr/local/bin/fin
ENTRYPOINT ["/usr/local/bin/fin"]
