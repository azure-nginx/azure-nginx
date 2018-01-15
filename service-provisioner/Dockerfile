FROM golang:1.8
RUN mkdir -p /app
WORKDIR /app
ADD service-provisioner /app
ADD templates /app/templates
CMD ["./service-provisioner"]