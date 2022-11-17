FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-mysql"]
COPY baton-mysql /