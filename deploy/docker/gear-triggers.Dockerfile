FROM scratch

COPY gear-triggers /gear-triggers

USER 10001:10001
ENTRYPOINT ["/gear-triggers"]
