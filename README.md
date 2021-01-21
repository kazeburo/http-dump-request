# http-dump-request
http-dump-request server and docker container for test

## API

dump request

```
% curl http://localhost:3000/ 
GET / HTTP/1.1
Host: localhost:3000
Accept: */*
User-Agent: curl/7.64.1
```

health check

```
% curl http://localhost:3000/live
OK
```

## Run with docker

```
$ docker run -p 3000:3000 nomadscafe.sakuracr.jp/http-dump-request
```