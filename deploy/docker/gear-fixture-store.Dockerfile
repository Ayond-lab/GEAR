FROM scratch

COPY gear-fixture-store /gear-fixture-store

USER 10001:10001
ENTRYPOINT ["/gear-fixture-store"]
