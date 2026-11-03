FROM scratch
COPY bin/backend /backend
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD ["/backend", "--check"]
ENTRYPOINT ["/backend"]
