FROM scratch

COPY gear-audit /gear-audit

USER 10001:10001
ENTRYPOINT ["/gear-audit"]
