FROM scratch

COPY gear-webhooks /gear-webhooks

USER 65532:65532
ENTRYPOINT ["/gear-webhooks"]
