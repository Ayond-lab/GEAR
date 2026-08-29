FROM scratch

COPY gear-controllers /gear-controllers

USER 10001:10001
ENTRYPOINT ["/gear-controllers"]
