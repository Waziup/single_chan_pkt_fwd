# Single Channel Packet Forwarder

## Build

Install go from https://golang.org/.

```sh
go build
```

To start the forwarder:

```sh
./single_chan_pkt_fwd
```

## Configuration

See [global_conf.json](https://github.com/Waziup/single_chan_pkt_fwd/blob/master/global_conf.json).

## Build the Docker Image

```sh
docker build --rm -f "Dockerfile" -t single_chan_pkt_fwd:latest "."
```

## docker-compose.yml

```yml
version: "3"
services:
  single_chan_pkt_fwd:
    container_name: single_chan_pkt_fwd
    image: waziup/single_chan_pkt_fwd
    build:
      context: ./single_chan_pkt_fwd
      dockerfile: Dockerfile
    volumes:
      - ./conf/single_chan_pkt_fwd:/etc/single_chan_pkt_fwd
      - /var/run/dbus:/var/run/dbus
      - /sys/class/gpio:/sys/class/gpio
      - /dev:/dev
    privileged: true
```

## Extract single_chan_pkt_fwd from Docker

```sh
docker build --rm -f "Dockerfile" -t single_chan_pkt_fwd:latest "."
id=$(docker create single_chan_pkt_fwd)
docker cp $id:/root/single_chan_pkt_fwd - > single_chan_pkt_fwd
docker rm -v $id
```