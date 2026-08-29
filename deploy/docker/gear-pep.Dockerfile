FROM scratch
COPY gear-pep /gear-pep
USER 1337:1337
ENTRYPOINT ["/gear-pep"]
