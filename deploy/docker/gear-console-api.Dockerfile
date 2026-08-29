FROM scratch

COPY gear-console-api /gear-console-api
COPY console /console

ENV GEAR_CONSOLE_DIR=/console
USER 10001:10001
ENTRYPOINT ["/gear-console-api"]
