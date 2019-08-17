FROM alpine:latest

ADD env-webhook /env-webhook
ENTRYPOINT ["./env-webhook"]