FROM gcr.io/distroless/static-debian12:nonroot
COPY gear-mandate /gear-mandate
ENTRYPOINT ["/gear-mandate"]
