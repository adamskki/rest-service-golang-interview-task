# Rest service in go - recruitment task

Rest service expose one HTTP endpoint supporting the following GET operation:
```
/random/mean?requests={r}&length={l}
```
which performs {r} concurrent requests to random.org API asking for {l} number of random integers.
## Setup

To run this project, enter the following commands

```
docker build . -t golang-rest-api:latest
docker run -d -p 8090:8090 golang-rest-api
```

Rest service will be available on 8090 port.