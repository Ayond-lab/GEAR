FROM scratch
COPY gear-hostile-runner /gear-hostile-runner
USER 1001:1001
ENTRYPOINT ["/gear-hostile-runner"]
