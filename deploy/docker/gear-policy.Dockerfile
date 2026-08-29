FROM scratch

COPY gear-policy /gear-policy

USER 10001:10001
ENTRYPOINT ["/gear-policy"]
