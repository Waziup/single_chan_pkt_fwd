FROM golang:1.12-alpine AS development

ENV PROJECT_PATH=/single_chan_pkt_fwd
ENV CGO_ENABLED=0

RUN apk add --no-cache ca-certificates git

COPY . $PROJECT_PATH
WORKDIR $PROJECT_PATH

RUN go build -o build/single_chan_pkt_fwd .

FROM alpine:latest AS production

RUN apk --no-cache add ca-certificates tzdata
COPY --from=development /single_chan_pkt_fwd/build/single_chan_pkt_fwd /root/single_chan_pkt_fwd
WORKDIR /etc/single_chan_pkt_fwd

ENTRYPOINT ["/root/single_chan_pkt_fwd"]
