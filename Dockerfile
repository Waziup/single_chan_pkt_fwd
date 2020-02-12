FROM golang:1.12-alpine AS development

ENV PROJECT_PATH=/single_chan_pkt_fwd
ENV PATH=$PATH:$PROJECT_PATH/build
ENV CGO_ENABLED=0

RUN apk add --no-cache ca-certificates git \
    && mkdir -p $PROJECT_PATH

COPY . $PROJECT_PATH
WORKDIR $PROJECT_PATH

RUN go build -o build/single_chan_pkt_fwd .

FROM alpine:latest AS production

WORKDIR /root/
RUN apk --no-cache add ca-certificates tzdata
COPY --from=development /single_chan_pkt_fwd/build/single_chan_pkt_fwd .
ENTRYPOINT ["./single_chan_pkt_fwd"]
