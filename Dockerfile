FROM gcr.io/distroless/base-debian10

COPY ./bin/mutation-webhook /

ENTRYPOINT [ "/mutation-webhook" ]
